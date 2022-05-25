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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	machineconfigcontroller "github.com/openshift/node-observability-operator/pkg/operator/controller/machineconfig"
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
	Log           logr.Logger
	MCOReconciler *machineconfigcontroller.MachineConfigReconciler
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
	r.Log, err = logr.FromContext(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Log.V(3).Info("Beginning Reconciliation")

	instance := &nodeobservabilityv1alpha1.NodeObservabilityRun{}
	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
			return
		}
		r.Log.Error(err, "failed to get NodeObservabilityRun")
		return
	}

	// interact with MCO only if we are executing CrioKubeletProfile runs
	// also bypass if the runtype is e2e-test
	if instance.Spec.RunType == nodeobservabilityv1alpha1.CrioKubeletProfile {
		if err := r.ensureNOMC(ctx); err != nil {
			return ctrl.Result{}, err
		}
		// this will ensure we dont trigger the agent call
		// until the mco and mcp have been updated successfully

		enabled, err := r.checkNOMCStatus(ctx, true)
		if err != nil {
			r.Log.Error(err, "NodeObservabilityMachineConfig enable status check failed")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if enabled {
			r.Log.V(3).Info("NodeObservabilityMachineConfig CrioKubeletProfile enabled")
		} else {
			r.Log.Info("NodeObservabilityMachineConfig CrioKubeletProfile not enabled yet")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	// this code should be included in the 'if' statement above
	// as it is specific to CrioKubeletProfiling
	// once new profiling features are added it can be moved

	// continue once machineconfigpool has been updated successfully
	if finished(instance) {
		r.Log.V(3).Info("Run for this instance has been completed already")
		// check the CR if the RestoreMCOStateAfterRun flag is set
		if instance.Spec.RestoreMCOStateAfterRun {
			err := r.disableCrioKubeletProfile(ctx)
			if err != nil {
				return ctrl.Result{}, err
			}

			disabled, err := r.checkNOMCStatus(ctx, false)
			if err != nil {
				r.Log.Error(err, "NodeObservabilityMachineConfig disable status check failed")
				return ctrl.Result{}, err
			}
			if disabled {
				r.Log.Info("NodeObservabilityMachineConfig CrioKubeletProfile disabled")
				nomc := r.desiredMCO()
				if err := r.deleteNOMC(ctx, nomc); err != nil {
					if !errors.IsNotFound(err) {
						return ctrl.Result{}, err
					}
				}
			} else {
				r.Log.Info("NodeObservabilityMachineConfig CrioKubeletProfile not disabled yet")
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			}
		}
		return
	}

	defer func() {
		errUpdate := r.Status().Update(ctx, instance, &client.UpdateOptions{})
		if errUpdate != nil {
			r.Log.Error(err, "failed to update status")
			err = utilerrors.NewAggregate([]error{err, errUpdate})
		}
	}()

	if inProgress(instance) {
		r.Log.V(3).Info("Run is in progress")
		var requeue bool
		requeue, err = r.handleInProgress(instance)
		if requeue {
			return ctrl.Result{RequeueAfter: pollingPeriod}, err
		}
		t := metav1.Now()
		instance.Status.FinishedTimestamp = &t
		return
	}

	err = r.startRun(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// one cycle takes cca 30s
	return ctrl.Result{RequeueAfter: time.Second * 30}, err
}

func (r *NodeObservabilityRunReconciler) handleInProgress(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) (bool, error) {
	var errors []error
	for _, agent := range instance.Status.Agents {
		url := r.format(agent.IP, r.AgentName, r.Namespace, pprofStatus, agent.Port)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityRunErrorRetriable, r.httpGetCall(url))
		if err != nil {
			if e, ok := err.(NodeObservabilityRunError); ok && e.HttpCode == http.StatusConflict {
				r.Log.V(3).Info("Received 407:StatusConflict, job still running")
				return true, nil
			}
			r.Log.Error(err, "failed to get agent status", "name", agent.Name, "IP", agent.IP)
			errors = append(errors, err)
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
		r.Log.V(3).Info("Initiating new run for node", "IP", a.IP, "port", port, "URL", url)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityRunErrorRetriable, r.httpGetCall(url))
		if err != nil {
			r.Log.Error(err, "failed to start profiling", "removing node from list", a.TargetRef.Name, "IP", a.IP)
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
