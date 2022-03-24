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
)

const (
	pollingPeriod = time.Second * 5
)

// NodeObservabilityRunReconciler reconciles a NodeObservabilityRun object
type NodeObservabilityRunReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
	AgentName string
}

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

// Reconcile manages NodeObservabilityRuns
func (r *NodeObservabilityRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	r.Log.V(3).Info("Reconcile", "Instance", req.NamespacedName.String())
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

	if finished(instance) {
		r.Log.V(3).Info("Finished", "Instance", req.NamespacedName.String())
		return
	}

	defer func() {
		errUpdate := r.Status().Update(ctx, instance, &client.UpdateOptions{})
		if errUpdate != nil {
			err = utilerrors.NewAggregate([]error{err, errUpdate})
		}
	}()

	if inProgress(instance) {
		r.Log.V(3).Info("In Progress", "Instance", req.NamespacedName.String())
		var requeue bool
		requeue, err = r.handleInProgress(instance)
		if requeue {
			return ctrl.Result{RequeueAfter: pollingPeriod}, err
		}
		t := metav1.Now()
		instance.Status.FinishedTimestamp = &t
		return
	}

	targets, err := r.startRun(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	instance.Status.Agents = targets
	t := metav1.Now()
	instance.Status.StartTimestamp = &t

	// one cycle takes cca 30s
	return ctrl.Result{RequeueAfter: time.Second * 30}, err
}

func (r *NodeObservabilityRunReconciler) handleInProgress(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) (bool, error) {
	var errors []error
	for _, agent := range instance.Status.Agents {
		url := fmt.Sprintf("http://%s:%d/status", agent.IP, agent.Port)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityErrorRetriable, httpGetCall(url))
		if err != nil {
			r.Log.Error(err, "failed to get agent status", "name", agent.Name, "IP", agent.IP)
			errors = append(errors, err)
			// TODO: node consistently failing, restart whole process
			continue
		}
	}
	return false, utilerrors.NewAggregate(errors)
}

func (r *NodeObservabilityRunReconciler) startRun(ctx context.Context, instance *nodeobservabilityv1alpha1.NodeObservabilityRun) ([]nodeobservabilityv1alpha1.AgentNode, error) {
	r.Log.V(3).Info("startRun", "instance", instance.Name)
	endps, err := r.getAgentEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	subset := endps.Subsets[0]
	port := subset.Ports[0].Port

	targets := []nodeobservabilityv1alpha1.AgentNode{}
	for _, a := range subset.Addresses {
		r.Log.V(3).Info("startRun - address for loop", "IP", a.IP)
		url := fmt.Sprintf("http://%s:%d/pprof", a.IP, port)
		err := retry.OnError(retry.DefaultBackoff, IsNodeObservabilityErrorRetriable, httpGetCall(url))
		if err != nil {
			r.Log.Error(err, "failed to start profiling", "removing node from list", a.TargetRef.Name, "IP", a.IP)
			// TODO: node consistently failing, restart whole process
			continue
		}
		r.Log.V(3).Info("startRun - address for loop", "code")
		targets = append(targets, nodeobservabilityv1alpha1.AgentNode{Name: a.TargetRef.Name, IP: a.IP, Port: port})
	}

	return targets, nil
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

func httpGetCall(url string) func() error {
	return func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		client := http.Client{Timeout: time.Second * 10}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return NodeObservabilityError{HttpCode: resp.StatusCode, Msg: string(body)}
		}
		return nil
	}
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodeobservabilityv1alpha1.NodeObservabilityRun{}).
		Complete(r)
}
