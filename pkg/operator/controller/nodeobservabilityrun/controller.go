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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

// NodeObservabilityRunReconciler reconciles a NodeObservabilityRun object
type NodeObservabilityRunReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilityruns/finalizers,verbs=update

// Reconcile manages NodeObservabilityRuns
func (r *NodeObservabilityRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &nodeobservabilityv1alpha1.NodeObservabilityRun{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if finished(instance) {
		return ctrl.Result{}, nil
	}

	if inProgress(instance) {
		err = r.handleInProgress(instance)
		return ctrl.Result{}, err
	}

	err = r.startRun(instance)

	return ctrl.Result{}, err
}

func (r *NodeObservabilityRunReconciler) handleInProgress(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) error {
	return nil
}

func (r *NodeObservabilityRunReconciler) startRun(instance *nodeobservabilityv1alpha1.NodeObservabilityRun) error {
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

// SetupWithManager sets up the controller with the Manager.
func (r *NodeObservabilityRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodeobservabilityv1alpha1.NodeObservabilityRun{}).
		Complete(r)
}
