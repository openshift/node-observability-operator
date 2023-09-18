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
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nodeobservabilityv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	opctrl "github.com/openshift/node-observability-operator/pkg/operator/controller"
)

const (
	pollingPeriod      = time.Second * 5
	httpRequestTimeout = time.Second * 10
	authHeader         = "Authorization"
	// nolint - ignore G101: not applicable
	pprofPath             = "node-observability-pprof"
	scriptPath            = "node-observability-scripting"
	pprofStatus           = "node-observability-status"
	nodeObservabilityKind = "NodeObservability"
)

// NodeObservabilityRunReconciler reconciles a NodeObservabilityRun object
type NodeObservabilityRunReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
	AuthToken []byte
	CACert    *x509.CertPool
}

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

// Reconcile manages NodeObservabilityRuns
func (r *NodeObservabilityRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	if ctxLog, err := logr.FromContext(ctx); err == nil {
		r.Log = ctxLog
	}

	r.Log.V(1).Info("reconciliation started")

	instance := &nodeobservabilityv1alpha2.NodeObservabilityRun{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			r.Log.V(1).Info("nodeobservabilityrun resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get nodeobservabilityrun: %w", err)
	}

	if runFinished(instance) {
		r.Log.V(1).Info("run for this instance has been completed already")
		return ctrl.Result{}, nil
	}

	if updated, err := r.adoptResource(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while setting owner reference NodeObservabilityRun->NodeObservability: %w", err)
	} else if updated {
		r.Log.V(1).Info("instance has just been adopted by nodeobservability", "nob.name", instance.Spec.NodeObservabilityRef.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	defer func() {
		// using named return values to handle status update in deferred function
		errUpdate := r.updateStatus(ctx, instance)
		if errUpdate != nil {
			errUpdate = fmt.Errorf("failed to update status: %w", errUpdate)
			err = utilerrors.NewAggregate([]error{err, errUpdate})
		}
	}()

	if canProceed, err := r.preconditionsMet(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check the preconditions: %w", err)
	} else if !canProceed {
		r.Log.V(1).Info("waiting for nodeobservability to become ready", "nob.name", instance.Spec.NodeObservabilityRef.Name)
		instance.Status.SetCondition(nodeobservabilityv1alpha2.DebugReady, metav1.ConditionFalse, nodeobservabilityv1alpha2.ReasonInProgress, fmt.Sprintf("Waiting for NodeObservability %s to become ready", instance.Spec.NodeObservabilityRef.Name))
		return ctrl.Result{RequeueAfter: pollingPeriod}, nil
	}

	instance.Status.SetCondition(nodeobservabilityv1alpha2.DebugReady, metav1.ConditionTrue, nodeobservabilityv1alpha2.ReasonReady, "Ready to start profiling")

	if runInProgress(instance) {
		r.Log.V(1).Info("run is in progress")
		var requeue bool
		if requeue, err = r.handleInProgress(instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to poll agents: %w", err)
		} else if requeue {
			instance.Status.SetCondition(nodeobservabilityv1alpha2.DebugFinished, metav1.ConditionFalse, nodeobservabilityv1alpha2.ReasonInProgress, "Profiling query in progress")
			return ctrl.Result{RequeueAfter: pollingPeriod}, err
		}
		r.Log.V(1).Info("execution query is done")
		t := metav1.Now()
		instance.Status.FinishedTimestamp = &t
		instance.Status.SetCondition(nodeobservabilityv1alpha2.DebugFinished, metav1.ConditionTrue, nodeobservabilityv1alpha2.ReasonFinished, "Profiling query done")
		return ctrl.Result{}, nil
	}

	r.Log.V(1).Info("ready to initiate an execution query")
	if err := r.startRun(ctx, instance); err != nil {
		instance.Status.SetCondition(nodeobservabilityv1alpha2.DebugFinished, metav1.ConditionFalse, nodeobservabilityv1alpha2.ReasonFailed, fmt.Sprintf("Failed to initiate profiling query: %s", err))
		return ctrl.Result{}, fmt.Errorf("failed to initiate an execution query: %w", err)
	}

	r.Log.V(1).Info("execution query initiated")
	instance.Status.SetCondition(nodeobservabilityv1alpha2.DebugFinished, metav1.ConditionFalse, nodeobservabilityv1alpha2.ReasonInProgress, "Profiling query initiated")

	// one cycle takes cca 30s
	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// adoptResource sets OwnerReference to point to NodeObservability from .spec.nodeobservabilityref.name.
// returns true if it was adopted by this function, false otherwise.
func (r *NodeObservabilityRunReconciler) adoptResource(ctx context.Context, instance *nodeobservabilityv1alpha2.NodeObservabilityRun) (bool, error) {
	for _, ref := range instance.OwnerReferences {
		if ref.Kind == nodeObservabilityKind && ref.Name == instance.Spec.NodeObservabilityRef.Name {
			return false, nil
		}
	}
	nodeObs := &nodeobservabilityv1alpha2.NodeObservability{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.NodeObservabilityRef.Name}, nodeObs); err != nil {
		return false, err
	}
	cpy := instance.DeepCopy()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, corErr := ctrlutil.CreateOrUpdate(ctx, r.Client, cpy, func() error {
			return ctrlutil.SetOwnerReference(nodeObs, cpy, r.Scheme)
		})
		return corErr
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *NodeObservabilityRunReconciler) handleInProgress(instance *nodeobservabilityv1alpha2.NodeObservabilityRun) (bool, error) {
	var errors []error
	for _, agent := range instance.Status.Agents {
		url := formatURL(agent.IP, opctrl.AgentServiceName, r.Namespace, agent.Port, pprofStatus)
		if err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityRunErrorRetriable, r.httpGetCall(url)); err != nil {
			if e, ok := err.(NodeObservabilityRunError); ok && e.HttpCode == http.StatusConflict {
				r.Log.V(1).Info("received 407:statusconflict, job still running", "agent.name", agent.Name, "agent.ip", agent.IP)
				return true, nil
			}
			errors = append(errors, fmt.Errorf("failed to get the status of the agent named %q with %q IP: %w", agent.Name, agent.IP, err))
			handleFailingAgent(instance, agent)
			continue
		}
	}
	return false, utilerrors.NewAggregate(errors)
}

func (r *NodeObservabilityRunReconciler) startRun(ctx context.Context, instance *nodeobservabilityv1alpha2.NodeObservabilityRun) error {
	endps, err := r.getAgentEndpoints(ctx)
	if err != nil {
		return err
	}
	subset := endps.Subsets[0]
	port := subset.Ports[0].Port

	targets := []nodeobservabilityv1alpha2.AgentNode{}
	failedTargets := []nodeobservabilityv1alpha2.AgentNode{}
	for _, a := range subset.NotReadyAddresses {
		failedTargets = append(failedTargets, nodeobservabilityv1alpha2.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
	}

	var url string
	for _, a := range subset.Addresses {
		if instance.Spec.NodeObservabilityRef.Mode == "profiling" {
			url = formatURL(a.IP, opctrl.AgentServiceName, r.Namespace, port, pprofPath)
		} else {
			url = formatURL(a.IP, opctrl.AgentServiceName, r.Namespace, port, scriptPath)
		}
		r.Log.V(1).Info("initiating new run for node", "node.name", a.NodeName, "pod.name", a.TargetRef.Name, "pod.ip", a.IP, "pod.port", port, "url", url)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityRunErrorRetriable, r.httpGetCall(url))
		if err != nil {
			r.Log.V(1).Info("failed to execute, removing node from list", "node.name", a.NodeName, "pod.name", a.TargetRef.Name, "pod.ip", a.IP, "error", err)
			failedTargets = append(failedTargets, nodeobservabilityv1alpha2.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
			continue
		}
		targets = append(targets, nodeobservabilityv1alpha2.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
	}

	t := metav1.Now()
	instance.Status.StartTimestamp = &t
	instance.Status.Agents = targets
	instance.Status.FailedAgents = failedTargets
	return nil
}

func (r *NodeObservabilityRunReconciler) updateStatus(ctx context.Context, instance *nodeobservabilityv1alpha2.NodeObservabilityRun) error {
	key := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	freshRun := &nodeobservabilityv1alpha2.NodeObservabilityRun{}
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

func (r *NodeObservabilityRunReconciler) preconditionsMet(ctx context.Context, instance *nodeobservabilityv1alpha2.NodeObservabilityRun) (bool, error) {
	no := &nodeobservabilityv1alpha2.NodeObservability{}
	key := types.NamespacedName{Name: instance.Spec.NodeObservabilityRef.Name}
	if err := r.Get(ctx, key, no); err != nil {
		return false, err
	}
	return no.Status.IsReady(), nil
}

func (r *NodeObservabilityRunReconciler) httpGetCall(url string) func() error {
	return func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set(authHeader, fmt.Sprintf("Bearer %s", string(r.AuthToken)))
		client := http.Client{
			Timeout: httpRequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:                  r.CACert,
					MinVersion:               tls.VersionTLS12,
					PreferServerCipherSuites: true,
				},
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return NodeObservabilityRunError{HttpCode: resp.StatusCode, Msg: string(body)}
		}
		return nil
	}
}

func (r *NodeObservabilityRunReconciler) getAgentEndpoints(ctx context.Context) (*corev1.Endpoints, error) {
	endpoints := &corev1.Endpoints{}
	if err := r.Get(ctx, types.NamespacedName{Name: opctrl.AgentServiceName, Namespace: r.Namespace}, endpoints); err != nil {
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodeobservabilityv1alpha2.NodeObservabilityRun{}).
		Complete(r)
}

func handleFailingAgent(instance *nodeobservabilityv1alpha2.NodeObservabilityRun, old nodeobservabilityv1alpha2.AgentNode) {
	var newAgents []nodeobservabilityv1alpha2.AgentNode
	for _, a := range instance.Status.Agents {
		if a.Name == old.Name {
			instance.Status.FailedAgents = append(instance.Status.FailedAgents, old)
		} else {
			newAgents = append(newAgents, a)
		}
	}
	instance.Status.Agents = newAgents
}

func runFinished(instance *nodeobservabilityv1alpha2.NodeObservabilityRun) bool {
	return instance.Status.FinishedTimestamp != nil && !instance.Status.FinishedTimestamp.IsZero()
}

func runInProgress(instance *nodeobservabilityv1alpha2.NodeObservabilityRun) bool {
	return instance.Status.StartTimestamp != nil && !instance.Status.StartTimestamp.IsZero()
}

func formatURL(ip, svcName, namespace string, port int32, path string) string {
	// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records-1
	// IP address won't pass TLS verification, agent serves certificate with DNS names as SAN
	podDNSName := fmt.Sprintf("%s.%s.%s.svc", strings.ReplaceAll(ip, ".", "-"), svcName, namespace)
	return (&url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(podDNSName, strconv.Itoa(int(port))),
		Path:   path,
	}).String()

}
