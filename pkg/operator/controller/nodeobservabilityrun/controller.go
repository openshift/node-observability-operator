/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeobservabilityruncontroller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	pollingPeriod    = time.Second * 5
	authHeader       = "Authorization"
	ProfilingMCPName = "nodeobservability"
	pprofPath        = "node-observability-pprof"
	pprofStatus      = "node-observability-status"
)

var (
	transport = http.DefaultTransport
)

// NodeObservabilityRunReconciler reconciles a NodeObservabilityRun object
type NodeObservabilityRunReconciler struct {
	client.Client
	Log logr.Logger
	URL
	Scheme    *runtime.Scheme
	Namespace string
	AgentName string
	AuthToken []byte
	CACert    *x509.CertPool
}

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

// Reconcile manages NodeObservabilityRuns
func (r *NodeObservabilityRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	var msg string
	if ctxLog, err := logr.FromContext(ctx); err == nil {
		r.Log = ctxLog
	}
	r.Log.V(1).Info("Beginning Reconciliation")

	instance := &nodeobservabilityv1alpha1.NodeObservabilityRun{}
	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
			return
		}
		err = fmt.Errorf("failed to get NodeObservabilityRun: %w", err)
		return
	}

	if finished(instance) {
		r.Log.V(1).Info("Run for this instance has been completed already")
		return
	}

	defer func() {
		errUpdate := r.updateStatus(ctx, instance)
		if errUpdate != nil {
			errUpdate = fmt.Errorf("failed to update status: %w", errUpdate)
			err = utilerrors.NewAggregate([]error{err, errUpdate})
		}
	}()

	var isAdopted bool
	if isAdopted, err = r.adoptResource(ctx, instance); !isAdopted {
		if err != nil {
			err = fmt.Errorf("error while setting owner reference NodeObservabilityRun->NodeObservability: %w", err)
			return
		}
		res = ctrl.Result{Requeue: true}
		return
	}

	var canProceed bool
	if canProceed, err = r.preconditionsMet(ctx, instance); !canProceed {
		if err != nil {
			err = fmt.Errorf("preconditions not met: %w", err)
			return
		}
		msg = fmt.Sprintf("Waiting for NodeObservability %s to become ready", instance.Spec.NodeObservabilityRef.Name)
		instance.Status.SetCondition(nodeobservabilityv1alpha1.DebugReady, metav1.ConditionFalse, nodeobservabilityv1alpha1.ReasonInProgress, msg)
		return ctrl.Result{RequeueAfter: pollingPeriod}, err
	}
	msg = "Ready to start profiling"
	instance.Status.SetCondition(nodeobservabilityv1alpha1.DebugReady, metav1.ConditionTrue, nodeobservabilityv1alpha1.ReasonReady, msg)

	if inProgress(instance) {
		r.Log.V(1).Info("Run is in progress")
		var requeue bool
		requeue, err = r.handleInProgress(instance)
		if requeue {
			msg = "Profiling query in progress"
			instance.Status.SetCondition(nodeobservabilityv1alpha1.DebugFinished, metav1.ConditionFalse, nodeobservabilityv1alpha1.ReasonInProgress, msg)
			return ctrl.Result{RequeueAfter: pollingPeriod}, err
		}
		t := metav1.Now()
		instance.Status.FinishedTimestamp = &t
		msg = "Profiling query done"
		instance.Status.SetCondition(nodeobservabilityv1alpha1.DebugFinished, metav1.ConditionTrue, nodeobservabilityv1alpha1.ReasonFinished, msg)
		return
	}

	err = r.startRun(ctx, instance)
	if err != nil {
		msg = fmt.Sprintf("Failed to initiate profiling query: %s", err.Error())
		instance.Status.SetCondition(nodeobservabilityv1alpha1.DebugFinished, metav1.ConditionFalse, nodeobservabilityv1alpha1.ReasonFailed, msg)
		return ctrl.Result{}, err
	}

	msg = "Profiling query initiated"
	instance.Status.SetCondition(nodeobservabilityv1alpha1.DebugFinished, metav1.ConditionFalse, nodeobservabilityv1alpha1.ReasonInProgress, msg)

	// one cycle takes cca 30s
	return ctrl.Result{RequeueAfter: time.Second * 30}, err
}

// adoptResource sets OwnerReference to point to NodeObservability from .spec.ref.name
// returns true if already adopted, false otherwise
func (r *NodeObservabilityRunReconciler) adoptResource(ctx context.Context, instance *nodeobservabilityv1alpha1.NodeObservabilityRun) (bool, error) {
	for _, ref := range instance.OwnerReferences {
		// FIXME: check kind as well?
		if ref.Name == instance.Spec.NodeObservabilityRef.Name {
			return true, nil
		}
	}
	nodeObs := &nodeobservabilityv1alpha1.NodeObservability{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.NodeObservabilityRef.Name}, nodeObs); err != nil {
		return false, err
	}
	cpy := instance.DeepCopy()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, corErr := ctrlutil.CreateOrUpdate(ctx, r.Client, cpy, func() error {
			return ctrlutil.SetOwnerReference(nodeObs, cpy, r.Scheme)
		})
		return corErr
	})
	return false, err
}

func (r *NodeObservabilityRunReconciler) handleInProgress(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) (bool, error) {
	var errors []error
	for _, agent := range instance.Status.Agents {
		url := r.format(agent.IP, r.AgentName, r.Namespace, pprofStatus, agent.Port)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityRunErrorRetriable, r.httpGetCall(url))
		if err != nil {
			if e, ok := err.(NodeObservabilityRunError); ok && e.HttpCode == http.StatusConflict {
				r.Log.V(1).Info("Received 407:StatusConflict, job still running")
				return true, nil
			}
			errors = append(errors, fmt.Errorf("failed to get the status of the agent named %q with %q IP: %w", agent.Name, agent.IP, err))
			handleFailingAgent(instance, agent)
			continue
		}
	}
	return false, utilerrors.NewAggregate(errors)
}

func (r *NodeObservabilityRunReconciler) startRun(ctx context.Context, instance *nodeobservabilityv1alpha1.NodeObservabilityRun) error {
	endps, err := r.getAgentEndpoints(ctx)
	if err != nil {
		return err
	}
	subset := endps.Subsets[0]
	port := subset.Ports[0].Port

	targets := []nodeobservabilityv1alpha1.AgentNode{}
	failedTargets := []nodeobservabilityv1alpha1.AgentNode{}
	for _, a := range subset.NotReadyAddresses {
		failedTargets = append(failedTargets, nodeobservabilityv1alpha1.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
	}

	for _, a := range subset.Addresses {
		url := r.format(a.IP, r.AgentName, r.Namespace, pprofPath, port)
		r.Log.V(1).Info("Initiating new run for node", "Name", a.TargetRef.Name, "IP", a.IP, "port", port, "URL", url)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityRunErrorRetriable, r.httpGetCall(url))
		if err != nil {
			r.Log.V(1).Info("Failed to start profiling, removing node from list", "Name", a.TargetRef.Name, "IP", a.IP, "Error", err)
			failedTargets = append(failedTargets, nodeobservabilityv1alpha1.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
			continue
		}
		targets = append(targets, nodeobservabilityv1alpha1.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
	}

	t := metav1.Now()
	instance.Status.StartTimestamp = &t
	instance.Status.Agents = targets
	instance.Status.FailedAgents = failedTargets
	return nil
}

func (r *NodeObservabilityRunReconciler) updateStatus(ctx context.Context, instance *nodeobservabilityv1alpha1.NodeObservabilityRun) error {
	key := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	freshRun := &nodeobservabilityv1alpha1.NodeObservabilityRun{}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, key, freshRun); err != nil {
			return err
		}
		if equality.Semantic.DeepEqual(freshRun.Status, instance.Status) {
			return nil
		}
		freshRun.Status = instance.Status
		return r.Status().Update(ctx, freshRun, &client.UpdateOptions{})
	})
}

func (r *NodeObservabilityRunReconciler) preconditionsMet(ctx context.Context, instance *nodeobservabilityv1alpha1.NodeObservabilityRun) (bool, error) {
	no := &nodeobservabilityv1alpha1.NodeObservability{}
	key := types.NamespacedName{Name: instance.Spec.NodeObservabilityRef.Name}
	if err := r.Get(ctx, key, no); err != nil {
		return false, err
	}
	return no.Status.IsReady(), nil
}

func finished(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) bool {
	t := instance.Status.FinishedTimestamp
	if t != nil && !t.IsZero() {
		return true
	}
	return false
}

func inProgress(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) bool {
	t := instance.Status.StartTimestamp
	if t != nil && !t.IsZero() {
		return true
	}
	return false
}

func (r *NodeObservabilityRunReconciler) httpGetCall(url string) func() error {
	return func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set(authHeader, fmt.Sprintf("Bearer %s", string(r.AuthToken)))
		client := http.Client{
			Timeout:   time.Second * 10,
			Transport: transport,
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return NodeObservabilityRunError{HttpCode: resp.StatusCode, Msg: string(body)}
		}
		return nil
	}
}

func handleFailingAgent(instance *nodeobservabilityv1alpha1.NodeObservabilityRun, old nodeobservabilityv1alpha1.AgentNode) {
	var newAgents []nodeobservabilityv1alpha1.AgentNode
	for _, a := range instance.Status.Agents {
		if a.Name == old.Name {
			instance.Status.FailedAgents = append(instance.Status.FailedAgents, old)
		} else {
			newAgents = append(newAgents, a)
		}
	}
	instance.Status.Agents = newAgents

}

func (r *NodeObservabilityRunReconciler) getAgentEndpoints(ctx context.Context) (*corev1.Endpoints, error) {
	endpoints := &corev1.Endpoints{}
	if err := r.Get(ctx, types.NamespacedName{Name: r.AgentName, Namespace: r.Namespace}, endpoints); err != nil {
		return nil, err
	}
	if len(endpoints.Subsets) != 1 {
		return nil, fmt.Errorf("wrong Endpoints.Subsets length, expected 1 item")
	}
	if len(endpoints.Subsets[0].Ports) != 1 {
		return nil, fmt.Errorf("wrong Endpoints.Subsets.Ports length, expected 1 item")
	}
	return endpoints, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeObservabilityRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{
		RootCAs:                  r.CACert,
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
	transport = t
	r.URL = &url{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodeobservabilityv1alpha1.NodeObservabilityRun{}).
		Complete(r)
}
