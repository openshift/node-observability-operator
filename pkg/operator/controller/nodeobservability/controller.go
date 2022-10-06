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

package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilclock "k8s.io/apimachinery/pkg/util/clock"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	logr "github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
)

const (
	finalizer = "NodeObservability"
	// the name of the NodeObservability resource which will be reconciled
	nodeObsCRName = "cluster"
)

var clock utilclock.Clock = utilclock.RealClock{}

// NodeObservabilityReconciler reconciles a NodeObservability object
type NodeObservabilityReconciler struct {
	client.Client
	ClusterWideClient client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	Namespace         string
	AgentImage        string
}

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilities/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=list;get;watch
//+kubebuilder:rbac:groups=core,resources=configmaps,resourceNames=kubelet-serving-ca,verbs=get;list;
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=list;get;
//+kubebuilder:rbac:groups=core,resources=nodes/proxy,verbs=list;get;
//+kubebuilder:rbac:urls=/debug/*,verbs=get;
//+kubebuilder:rbac:urls=/node-observability-status,verbs=get;
//+kubebuilder:rbac:urls=/node-observability-pprof,verbs=get;
//+kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create;
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create;
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get,resourceNames=node-observability-operator-agent
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=list;get;create;watch;delete;update;patch
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=list;get;create;watch;use;delete;update;patch
//+kubebuilder:rbac:groups=apps,namespace=node-observability-operator,resources=daemonsets,verbs=list;get;create;watch;update;patch
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=services,verbs=list;get;create;watch;delete;update;patch;
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=serviceaccounts,verbs=list;get;create;watch;delete;update;patch;
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=configmaps,verbs=list;get;create;watch;delete;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *NodeObservabilityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if ctxLog, err := logr.FromContext(ctx); err == nil {
		r.Log = ctxLog
	}

	r.Log.V(1).Info("reconciliation started")

	// Fetch the NodeObservability instance
	nodeObs := &operatorv1alpha2.NodeObservability{}
	err := r.Get(ctx, req.NamespacedName, nodeObs)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.V(1).Info("nodeobservability resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get nodeobservability: %w", err)
	}

	err = isClusterNodeObservability(ctx, nodeObs)
	if err != nil {
		// Update nodeObs Status
		nodeObs.Status.SetCondition(operatorv1alpha2.DebugReady, metav1.ConditionFalse, operatorv1alpha2.ReasonInvalid, err.Error())

		nodeObs.Status.Count = 0
		now := metav1.NewTime(clock.Now())
		nodeObs.Status.LastUpdate = &now

		// Call API Update Status
		errUpdate := r.Status().Update(ctx, nodeObs)
		if errUpdate != nil {
			errUpdate = fmt.Errorf("failed to update status for NodeObservability %v: %w", nodeObs, errUpdate)
			return ctrl.Result{}, utilerrors.NewAggregate([]error{err, errUpdate})
		}
		r.Log.V(3).Info("Status updated:", "Count", "0", "LastUpdated", now, "Ready", false)
		r.Log.Error(err, "exiting reconcile")
		// Return without err, to prevent requeuing
		return ctrl.Result{}, nil
	}

	// nodeObs is named cluster: proceed
	if nodeObs.DeletionTimestamp != nil {
		r.Log.V(1).Info("nodeobservability resource is going to be deleted. Taking action")
		if err := r.ensureNodeObservabilityDeleted(ctx, nodeObs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to ensure nodeobservability deletion: %w", err)
		}
		return ctrl.Result{}, nil

	}
	r.Log.V(1).Info("NodeObservability resource found", "Namespace", req.NamespacedName.Namespace, "Name", nodeObs.Name)

	// Set finalizers on the NodeObservability resource
	updated, err := r.withFinalizers(ctx, nodeObs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update nodeobservability with finalizers:, %w", err)
	}
	nodeObs = updated

	// For the pods to deploy on each node and execute the crio & kubelet script we need the following
	// - custom scc (mainly allowHostPathDirPlugin set to true)
	// - serviceaccount
	// - clusterrole (use the scc)
	// - clusterrolebinding (bind the sa to the role)

	// ensure scc
	scc, err := r.ensureSecurityContextConstraints(ctx, nodeObs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure securitycontectconstraints : %w", err)
	}
	r.Log.V(1).Info("securitycontextconstraint ensured", "scc.name", scc.Name)

	// ensure serviceaccount
	sa, err := r.ensureServiceAccount(ctx, nodeObs, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure serviceaccount : %w", err)
	}
	r.Log.V(1).Info("serviceaccount ensured", "sa.namespace", sa.Namespace, "sa.name", sa.Name)

	// ensure service
	svc, err := r.ensureService(ctx, nodeObs, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure service : %w", err)
	}
	r.Log.V(1).Info("service ensured", "svc.namespace", svc.Namespace, "svc.name", svc.Name)

	// verify if clusterrole exists
	exists, err := r.verifyClusterRole(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to verify clusterrole %s : %w", clusterRoleName, err)
	} else if !exists {
		return ctrl.Result{}, fmt.Errorf("clusterrole %q does not exist", clusterRoleName)
	}
	r.Log.V(1).Info("clusterrole ensured", "clusterrole.name", clusterRoleName)

	// ensure clusterolebinding with serviceaccount
	crb, err := r.ensureClusterRoleBinding(ctx, nodeObs, sa.Name, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure clusterrolebinding : %w", err)
	}
	r.Log.V(1).Info("clusterrolebinding ensured", "clusterrolebinding.name", crb.Name)

	// check daemonset
	ds, err := r.ensureDaemonSet(ctx, nodeObs, sa, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure daemonset : %w", err)
	}
	r.Log.V(1).Info("daemonset ensured", "ds.namespace", ds.Namespace, "ds.name", ds.Name)

	dsReady := ds.Status.NumberReady == ds.Status.DesiredNumberScheduled

	// if machine config change is not requested, we can mark it as ready
	var nomcReady bool = true
	if machineConfigChangeRequested(nodeObs) {
		nomc, err := r.ensureNOMC(ctx, nodeObs)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to ensure nodeobservabilitymachineconfig : %w", err)
		}
		r.Log.V(1).Info("nodeobservabilitymachineconfig ensured", "nomc.name", nomc.Name)
		nomcReady = nomc.Status.IsReady()
	}

	msg := fmt.Sprintf("DaemonSet %s ready: %t NodeObservabilityMachineConfig ready: %t", ds.Name, dsReady, nomcReady)
	if dsReady && nomcReady {
		nodeObs.Status.SetCondition(operatorv1alpha2.DebugReady, metav1.ConditionTrue, operatorv1alpha2.ReasonReady, msg)
	} else {
		nodeObs.Status.SetCondition(operatorv1alpha2.DebugReady, metav1.ConditionFalse, operatorv1alpha2.ReasonInProgress, msg)
	}

	nodeObs.Status.Count = ds.Status.NumberReady
	now := metav1.NewTime(clock.Now())
	nodeObs.Status.LastUpdate = &now
	err = r.Status().Update(ctx, nodeObs)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Log.V(1).Info("Status updated", "Count", ds.Status.NumberReady, "LastUpdated", now)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeObservabilityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha2.NodeObservability{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&operatorv1alpha2.NodeObservabilityMachineConfig{}).
		Complete(r)
}

func hasFinalizer(nodeObs *operatorv1alpha2.NodeObservability) bool {
	hasFinalizer := false
	for _, f := range nodeObs.Finalizers {
		if f == finalizer {
			hasFinalizer = true
			break
		}
	}
	return hasFinalizer
}

func (r *NodeObservabilityReconciler) withoutFinalizers(ctx context.Context, nodeObs *operatorv1alpha2.NodeObservability, finalizerFlag string) (*operatorv1alpha2.NodeObservability, error) {
	withoutFinalizers := nodeObs.DeepCopy()

	newFinalizers := make([]string, 0)
	for _, item := range withoutFinalizers.Finalizers {
		if item == finalizerFlag {
			continue
		}
		newFinalizers = append(newFinalizers, item)
	}
	if len(newFinalizers) == 0 {
		// Sanitize for unit tests so we don't need to distinguish empty array
		// and nil.
		newFinalizers = nil
	}
	withoutFinalizers.Finalizers = newFinalizers
	if err := r.Update(ctx, withoutFinalizers); err != nil {
		return withoutFinalizers, fmt.Errorf("failed to remove finalizers: %w", err)
	}
	return withoutFinalizers, nil
}

func (r *NodeObservabilityReconciler) withFinalizers(ctx context.Context, nodeObs *operatorv1alpha2.NodeObservability) (*operatorv1alpha2.NodeObservability, error) {
	withFinalizers := nodeObs.DeepCopy()

	if !hasFinalizer(withFinalizers) {
		withFinalizers.Finalizers = append(withFinalizers.Finalizers, finalizer)
	}

	if err := r.Update(ctx, withFinalizers); err != nil {
		return withFinalizers, fmt.Errorf("failed to update finalizers: %w", err)
	}
	return withFinalizers, nil
}

func (r *NodeObservabilityReconciler) ensureNodeObservabilityDeleted(ctx context.Context, nodeObs *operatorv1alpha2.NodeObservability) error {
	errs := []error{}

	if err := r.deleteClusterRoleBinding(nodeObs); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete clusterrolebinding : %w", err))
	}
	if err := r.deleteSecurityContextConstraints(nodeObs); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete SCC : %w", err))
	}
	if err := r.deleteNOMC(ctx, nodeObs); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete nodeobservabilitymachineconfig : %w", err))
	}
	if len(errs) == 0 && hasFinalizer(nodeObs) {
		// Remove the finalizer.
		_, err := r.withoutFinalizers(ctx, nodeObs, finalizer)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to remove finalizer from nodeobservability %s/%s: %w", nodeObs.Namespace, nodeObs.Name, err))
		}
	}
	return utilerrors.NewAggregate(errs)
}

// machineConfigChangeRequested returns true, when a given NodeObservabilityType needs
// machine config change. Only CrioKubeletNodeObservabilityType requires a MC change, false otherwise
func machineConfigChangeRequested(nodeObs *operatorv1alpha2.NodeObservability) bool {
	return nodeObs.Spec.Type == operatorv1alpha2.CrioKubeletNodeObservabilityType
}

func isClusterNodeObservability(ctx context.Context, nodeObs *operatorv1alpha2.NodeObservability) error {

	if nodeObs.Name == nodeObsCRName {
		return nil
	}

	return fmt.Errorf("a single NodeObservability with name 'cluster' is authorized. Resource %s will be ignored", nodeObs.Name)
}
