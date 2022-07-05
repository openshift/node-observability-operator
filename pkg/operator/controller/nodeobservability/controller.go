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

	logr "github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilclock "k8s.io/apimachinery/pkg/util/clock"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	finalizer = "NodeObservability"
	// the name of the NodeObservability resource which will be reconciled
	nodeObsCRName = "cluster"
)

var (
	clock utilclock.Clock = utilclock.RealClock{}
)

// NodeObservabilityReconciler reconciles a NodeObservability object
type NodeObservabilityReconciler struct {
	client.Client
	ClusterWideClient client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	Namespace         string
	AgentImage        string
	// Used to inject errors for testing
	Err ErrTestObject
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
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=list;get;create;watch;delete;
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=list;get;create;watch;delete;
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=list;get;create;watch;use;delete;
//+kubebuilder:rbac:groups=apps,namespace=node-observability-operator,resources=daemonsets,verbs=list;get;create;watch;
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=secrets,verbs=list;get;create;watch;delete;
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=services,verbs=list;get;create;watch;delete;
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=serviceaccounts,verbs=list;get;create;watch;delete;
//+kubebuilder:rbac:groups=core,namespace=node-observability-operator,resources=configmaps,verbs=list;get;create;watch;delete;update;
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace=node-observability-operator,resources=roles,verbs=list;get;create;watch;delete;
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace=node-observability-operator,resources=rolebindings,verbs=list;get;create;watch;delete;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *NodeObservabilityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	if ctxLog, err := logr.FromContext(ctx); err == nil {
		r.Log = ctxLog
	}

	// Fetch the NodeObservability instance
	nodeObs := &operatorv1alpha1.NodeObservability{}
	err := r.Get(ctx, req.NamespacedName, nodeObs)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.V(1).Info("NodeObservability resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get NodeObservability: %w", err)
	}
	r.Log.V(3).Info("Reconciling:", "NodeObservability", nodeObs)

	err = isClusterNodeObservability(ctx, nodeObs)
	if err != nil {
		// Update nodeObs Status
		nodeObs.Status.SetCondition(operatorv1alpha1.DebugReady, metav1.ConditionFalse, operatorv1alpha1.ReasonInvalid, err.Error())

		nodeObs.Status.Count = 0
		now := metav1.NewTime(clock.Now())
		nodeObs.Status.LastUpdate = &now

		// Call API Update Status
		errUpdate := r.Status().Update(ctx, nodeObs)
		if errUpdate != nil {
			errUpdate = fmt.Errorf("failed to update status for NodeObservability %v: %w", nodeObs, errUpdate)
			return ctrl.Result{}, utilerrors.NewAggregate([]error{err, errUpdate})
		}
		r.Log.V(3).Info("Status updated ", "Count", "0", "LastUpdated", now, "Ready", false)
		r.Log.Error(err, "Exiting reconcile")
		// Return without err, to prevent requeuing
		return ctrl.Result{}, nil
	}

	// nodeObs is named cluster: proceed
	if nodeObs.DeletionTimestamp != nil {
		r.Log.V(1).Info("NodeObservability resource is going to be deleted. Taking action")
		if err := r.ensureNodeObservabilityDeleted(ctx, nodeObs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to ensure nodeobservability deletion: %w", err)
		}
		return ctrl.Result{}, nil

	}
	r.Log.V(1).Info("NodeObservability resource found", "Namespace", req.NamespacedName.Namespace, "Name", nodeObs.Name)

	// Set finalizers on the NodeObservability resource
	updated, err := r.withFinalizers(ctx, nodeObs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update NodeObservability with finalizers:, %w", err)
	}
	nodeObs = updated

	// For the pods to deploy on each node and execute the crio & kubelet script we need the following
	// - custom scc (mainly allowHostPathDirPlugin set to true)
	// - serviceaccount
	// - clusterrole (use the scc)
	// - clusterrolebinding (bind the sa to the role)

	// ensure scc
	haveSCC, scc, err := r.ensureSecurityContextConstraints(ctx, nodeObs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure securitycontectconstraints : %w", err)
	} else if !haveSCC {
		return ctrl.Result{}, fmt.Errorf("failed to get securitycontextconstraints")
	}
	r.Log.V(1).Info("SecurityContextConstraints ensured", "Name", scc.Name)

	// ensure serviceaccount
	haveSA, sa, err := r.ensureServiceAccount(ctx, nodeObs, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure serviceaccount : %w", err)
	} else if !haveSA {
		return ctrl.Result{}, fmt.Errorf("failed to get serviceaccount")
	}
	r.Log.V(1).Info("ServiceAccount ensured", "Namespace", sa.Namespace, "Name", sa.Name)

	// ensure service
	haveSvc, svc, err := r.ensureService(ctx, nodeObs, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure service : %w", err)
	} else if !haveSvc {
		return ctrl.Result{}, fmt.Errorf("failed to get service")
	}
	r.Log.V(1).Info("Service ensured", "Namespace", svc.Namespace, "Name", svc.Name)

	// check clusterrole
	haveCR, cr, err := r.ensureClusterRole(ctx, nodeObs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure clusterrole : %w", err)
	} else if !haveCR {
		return ctrl.Result{}, fmt.Errorf("failed to get clusterrole")
	}
	r.Log.V(1).Info("ClusterRole ensured", "Name", cr.Name)

	// check clusterolebinding with serviceaccount
	haveCRB, crb, err := r.ensureClusterRoleBinding(ctx, nodeObs, sa.Name, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure clusterrolebinding : %w", err)
	} else if !haveCRB {
		return ctrl.Result{}, fmt.Errorf("failed to get clusterrolebinding")
	}
	r.Log.V(1).Info("ClusterRoleBinding ensured", "Name", crb.Name)

	// check daemonset
	haveDS, ds, err := r.ensureDaemonSet(ctx, nodeObs, sa, r.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure daemonset : %w", err)
	} else if !haveDS {
		return ctrl.Result{}, fmt.Errorf("failed to get daemonset")
	}
	r.Log.V(1).Info("DaemonSet ensured", "Namespace", ds.Namespace, "Name", ds.Name)

	dsReady := ds.Status.NumberReady == ds.Status.DesiredNumberScheduled

	// if machine config change is not requested, we can mark it as ready
	var nomcReady bool = true
	if machineConfigChangeRequested(nodeObs) {
		haveNOMC, nomc, err := r.ensureNOMC(ctx, nodeObs)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to ensure nodeobservabilitymachineconfig : %w", err)
		} else if !haveNOMC {
			return ctrl.Result{}, fmt.Errorf("failed to get nodeobservabilitymachineconfig")
		}
		r.Log.V(1).Info("NodeObservabilityMachineConfig ensured", "Name", nomc.Name)
		nomcReady = nomc.Status.IsReady()
	}

	msg := fmt.Sprintf("DaemonSet %s ready: %t NodeObservabilityMachineConfig ready: %t", ds.Name, dsReady, nomcReady)
	if dsReady && nomcReady {
		nodeObs.Status.SetCondition(operatorv1alpha1.DebugReady, metav1.ConditionTrue, operatorv1alpha1.ReasonReady, msg)
	} else {
		nodeObs.Status.SetCondition(operatorv1alpha1.DebugReady, metav1.ConditionFalse, operatorv1alpha1.ReasonInProgress, msg)
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
		For(&operatorv1alpha1.NodeObservability{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&operatorv1alpha1.NodeObservabilityMachineConfig{}).
		Complete(r)
}

func hasFinalizer(nodeObs *operatorv1alpha1.NodeObservability) bool {
	hasFinalizer := false
	for _, f := range nodeObs.Finalizers {
		if f == finalizer {
			hasFinalizer = true
			break
		}
	}
	return hasFinalizer
}

func (r *NodeObservabilityReconciler) withoutFinalizers(ctx context.Context, nodeObs *operatorv1alpha1.NodeObservability, finalizerFlag string) (*operatorv1alpha1.NodeObservability, error) {
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

func (r *NodeObservabilityReconciler) withFinalizers(ctx context.Context, nodeObs *operatorv1alpha1.NodeObservability) (*operatorv1alpha1.NodeObservability, error) {
	withFinalizers := nodeObs.DeepCopy()

	if !hasFinalizer(withFinalizers) {
		withFinalizers.Finalizers = append(withFinalizers.Finalizers, finalizer)
	}

	if err := r.Update(ctx, withFinalizers); err != nil {
		return withFinalizers, fmt.Errorf("failed to update finalizers: %w", err)
	}
	return withFinalizers, nil
}

func (r *NodeObservabilityReconciler) ensureNodeObservabilityDeleted(ctx context.Context, nodeObs *operatorv1alpha1.NodeObservability) error {
	errs := []error{}

	if err := r.deleteClusterRole(nodeObs); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete clusterrole : %w", err))
	}
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
func machineConfigChangeRequested(nodeObs *operatorv1alpha1.NodeObservability) bool {
	return nodeObs.Spec.Type == operatorv1alpha1.CrioKubeletNodeObservabilityType
}

func isClusterNodeObservability(ctx context.Context, nodeObs *operatorv1alpha1.NodeObservability) error {

	if nodeObs.Name == nodeObsCRName {
		return nil
	}

	return fmt.Errorf("a single NodeObservability with name 'cluster' is authorized. Resource %s will be ignored", nodeObs.Name)
}
