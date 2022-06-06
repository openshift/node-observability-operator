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

package machineconfigcontroller

import (
	"context"
	"fmt"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create

// New returns a new MachineConfigReconciler instance.
func New(mgr ctrl.Manager, impls ...impl) (*MachineConfigReconciler, error) {
	effectiveClient := NewClient(mgr, impls...)
	return &MachineConfigReconciler{
		impl: effectiveClient,

		Log:           ctrl.Log.WithName("controller").WithName("NodeObservabilityMachineConfig"),
		Scheme:        effectiveClient.ManagerGetScheme(mgr),
		EventRecorder: effectiveClient.ManagerGetEventRecorderFor(mgr, "node-observability-operator"),

		Node: NodeSyncData{
			PrevReconcileUpd: make(map[string]LabelInfo),
		},
		MachineConfig: MachineConfigSyncData{
			PrevReconcileUpd: make(map[string]MachineConfigInfo),
		},
	}, nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
//
// Reconcile here is for NodeObservabilityMachineConfig controller, which aims
// to keep the state as required by the NodeObservability operator. If for any
// service(ex: CRI-O) requires debugging to be enabled/disabled through the
// MachineConfigs, controller creates the required MachineConfigs, MachineConfigPool
// and labels the nodes where the changes are to be applied.
func (r *MachineConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {

	if r.Log, err = logr.FromContext(ctx); err != nil {
		return
	}

	r.Log.V(3).Info("Reconciling NodeObservabilityMachineConfig of Nodeobservability operator")

	// Fetch the nodeobservability.olm.openshift.io/nodeobservabilitymachineconfig CR
	r.CtrlConfig = &v1alpha1.NodeObservabilityMachineConfig{}
	if err = r.ClientGet(ctx, req.NamespacedName, r.CtrlConfig); err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("NodeObservabilityMachineConfig resource not found. Ignoring could have been deleted", "name", req.NamespacedName.Name)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "failed to fetch NodeObservabilityMachineConfig")
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
	}
	r.Log.V(3).Info("NodeObservabilityMachineConfig resource found")

	if !r.CtrlConfig.DeletionTimestamp.IsZero() {
		r.Log.Info("NodeObservabilityMachineConfig resource marked for deletion, cleaning up")
		result, err = r.cleanUp(ctx, req)

		if r.CtrlConfig.Status.IsMachineConfigInProgress() {
			r.CtrlConfig.Status.UpdateLastReconcileTime()
			if errUpdate := r.updateStatus(ctx); errUpdate != nil {
				r.Log.Error(err, "failed to update cleanup status")
				result = ctrl.Result{RequeueAfter: defaultRequeueTime}
				err = utilerrors.NewAggregate([]error{err, errUpdate})
			}
		}
		return
	}

	// Set finalizers on the NodeObservabilityMachineConfig resource
	updated, err := r.addFinalizer(ctx, req)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update NodeObservabilityMachineConfig with finalizers: %w", err)
	}
	updated.DeepCopyInto(r.CtrlConfig)

	if !r.CtrlConfig.Status.LastReconcile.IsZero() &&
		r.CtrlConfig.Status.IsMachineConfigInProgress() {
		diff := time.Since(r.CtrlConfig.Status.LastReconcile.Time).Round(time.Second)
		if diff < time.Minute && diff >= 0 {
			next := time.Minute - diff
			r.Log.Info("reconciler called earlier than expected", "next reconcile in", next.String())
			return ctrl.Result{Requeue: true, RequeueAfter: next}, nil
		}
	}

	defer func() {
		r.CtrlConfig.Status.UpdateLastReconcileTime()
		if errUpdate := r.updateStatus(ctx); errUpdate != nil {
			r.Log.Error(err, "failed to update status")
			result = ctrl.Result{RequeueAfter: defaultRequeueTime}
			err = utilerrors.NewAggregate([]error{err, errUpdate})
		}
	}()

	requeue, err := r.inspectProfilingMCReq(ctx)
	if err != nil {
		r.Log.Error(err, "failed to reconcile requested configuration")
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
	}
	// if the configuration changes were made in the current reconciling
	// will requeue to avoid the existing status of MCP to be considered
	// and allow MCO to pick the changes and update correct state
	if requeue {
		r.Log.Info("updated configurations, reconcile again in a minute")
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	return r.monitorProgress(ctx)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodeObservabilityMachineConfig{}).
		Owns(&mcv1.MachineConfig{}).
		Owns(&mcv1.MachineConfigPool{}).
		WithEventFilter(ignoreNOMCStatusUpdates()).
		Complete(r)
}

// ignoreNOMCStatusUpdates is for ignoring NodeObservabilityMachineConfig
// resource status update events and for not adding to the reconcile queue
func ignoreNOMCStatusUpdates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// old object does not exist, nothing to update
			if e.ObjectOld == nil {
				return false
			}
			// new object does not exist, nothing to update
			if e.ObjectNew == nil {
				return false
			}

			// if NOMC resource version is not changed, it indicates
			// spec or metadata has not changed and the event could be for
			// status update which need not be queued for reconciliation
			if _, ok := e.ObjectOld.(*v1alpha1.NodeObservabilityMachineConfig); ok {
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			}
			return true
		},
	}
}

// cleanUp is handling deletion of NodeObservabilityMachineConfig resource
// deletion. Reverts all the changes made when debugging is enabled and
// restores the cluster to earlier state.
func (r *MachineConfigReconciler) cleanUp(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	disabled, err := r.ensureProfConfDisabled(ctx)
	if err != nil {
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
	}
	if disabled {
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	if r.CtrlConfig.Status.IsMachineConfigInProgress() {
		if result, err := r.checkWorkerMCPStatus(ctx); err != nil {
			return result, err
		}
	}

	removeFinalizer := false
	if !r.CtrlConfig.Status.IsMachineConfigInProgress() {
		if !r.CtrlConfig.Status.IsDebuggingEnabled() {
			removeFinalizer = true
			r.Log.Info("disable debug successful for cleanup")
		}

		if r.CtrlConfig.Status.IsDebuggingFailed() {
			removeFinalizer = true
			r.Log.Info("failed to disable debug for cleanup")
		}
	}

	if removeFinalizer && hasFinalizer(r.CtrlConfig) {
		if _, err := r.removeFinalizer(ctx, req, finalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from NodeObservabilityMachineConfig %s: %w", r.CtrlConfig.Name, err)
		}
		r.Log.Info("removed finalizer from NodeObservabilityMachineConfig resource, cleanup complete")
	}

	return ctrl.Result{}, nil
}

// addFinalizer adds finalizer to NodeObservabilityMachineConfig resource
// if does not exist
func (r *MachineConfigReconciler) addFinalizer(ctx context.Context, req ctrl.Request) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	withFinalizers := &v1alpha1.NodeObservabilityMachineConfig{}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.ClientGet(ctx, req.NamespacedName, withFinalizers); err != nil {
			r.Log.Error(err, "failed to fetch nodeobservabilitymachineconfig resource for updating finalizer")
			return err
		}

		if hasFinalizer(withFinalizers) {
			return nil
		}
		withFinalizers.Finalizers = append(withFinalizers.Finalizers, finalizer)

		if err := r.ClientUpdate(ctx, withFinalizers); err != nil {
			r.Log.Error(err, "failed to update nodeobservabilitymachineconfig resource finalizers")
			return err
		}

		return nil
	})

	return withFinalizers, err
}

// removeFinalizer removes finalizers added to
// NodeObservabilityMachineConfig resource if present
func (r *MachineConfigReconciler) removeFinalizer(ctx context.Context, req ctrl.Request, finalizer string) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	withoutFinalizers := &v1alpha1.NodeObservabilityMachineConfig{}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.ClientGet(ctx, req.NamespacedName, withoutFinalizers); err != nil {
			r.Log.Error(err, "failed to fetch nodeobservabilitymachineconfig resource for removing finalizer")
			return err
		}

		if !hasFinalizer(withoutFinalizers) {
			return nil
		}

		newFinalizers := make([]string, 0)
		for _, item := range withoutFinalizers.Finalizers {
			if item == finalizer {
				continue
			}
			newFinalizers = append(newFinalizers, item)
		}

		if len(newFinalizers) == 0 {
			// Sanitize for unit tests, so we don't need to distinguish empty array
			// and nil.
			newFinalizers = nil
		}

		withoutFinalizers.Finalizers = newFinalizers
		if err := r.ClientUpdate(ctx, withoutFinalizers); err != nil {
			r.Log.Error(err, "failed to remove nodeobservabilitymachineconfig resource finalizers")
			return err
		}

		return nil
	})

	return withoutFinalizers, err
}

// hasFinalizer checks if the required finalizer is present
// in the NodeObservabilityMachineConfig resource
func hasFinalizer(mc *v1alpha1.NodeObservabilityMachineConfig) bool {
	hasFinalizer := false
	for _, f := range mc.Finalizers {
		if f == finalizer {
			hasFinalizer = true
			break
		}
	}
	return hasFinalizer
}

// updateStatus is updating the status subresource of NodeObservabilityMachineConfig
func (r *MachineConfigReconciler) updateStatus(ctx context.Context) error {

	namespace := types.NamespacedName{Name: r.CtrlConfig.Name}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.Log.V(3).Info("updating nodeobservabilitymachineconfig resource status")
		nomc := &v1alpha1.NodeObservabilityMachineConfig{}
		if err := r.ClientGet(ctx, namespace, nomc); err != nil {
			r.Log.Error(err, "failed to fetch nodeobservabilitymachineconfig resource")
			return err
		}
		r.CtrlConfig.Status.DeepCopyInto(&nomc.Status)

		if err := r.ClientStatusUpdate(ctx, nomc); err != nil {
			r.Log.Error(err, "failed to update nodeobservabilitymachineconfig resource status")
			return err
		}
		nomc.DeepCopyInto(r.CtrlConfig)

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// inspectProfilingMCReq is for checking and creating required configs
// if debugging is enabled
func (r *MachineConfigReconciler) inspectProfilingMCReq(ctx context.Context) (bool, error) {
	if r.CtrlConfig.Status.IsMachineConfigInProgress() {
		r.Log.V(3).Info("previous reconcile initiated operation in progress, changes not applied")
		return false, nil
	}

	if r.CtrlConfig.Spec.Debug.EnableCrioProfiling {
		return r.ensureProfConfEnabled(ctx)
	} else {
		return r.ensureProfConfDisabled(ctx)
	}
}

// ensureProfConfEnabled is for enabling the profiling of requested services
func (r *MachineConfigReconciler) ensureProfConfEnabled(ctx context.Context) (bool, error) {

	var modCount, setEnabledCondition int
	var err error
	if modCount, err = r.ensureReqNodeLabelExists(ctx); err != nil {
		r.Log.Error(err, "failed to ensure nodes are labelled")
		// fails for even one node revert changes made
		return true, r.revertNodeLabeling(ctx)
	}
	setEnabledCondition += modCount
	if modCount, err = r.ensureReqMCPExists(ctx); err != nil {
		r.Log.Error(err, "failed to ensure mcp exists")
		return false, err
	}
	setEnabledCondition += modCount
	if modCount, err = r.ensureReqMCExists(ctx); err != nil {
		r.Log.Error(err, "failed to ensure mc exists")
		return false, err
	}
	setEnabledCondition += modCount

	if setEnabledCondition > 0 {
		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled, metav1.ConditionTrue, v1alpha1.ReasonEnabled,
			"debug configurations enabled")
		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress,
			"applying debug configurations in progress")
		return true, nil
	}

	return false, nil
}

// ensureProfConfDisabled is for disabling the profiling of requested services
func (r *MachineConfigReconciler) ensureProfConfDisabled(ctx context.Context) (bool, error) {

	modCount := 0
	var err error
	if modCount, err = r.ensureReqNodeLabelNotExists(ctx); err != nil {
		r.Log.Error(err, "failed to ensure nodes are not labelled")
		// fails for even one node revert changes made
		return true, r.revertNodeUnlabeling(ctx)
	}

	if modCount > 0 {
		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled, metav1.ConditionFalse, v1alpha1.ReasonDisabled,
			"debug configurations disabled")
		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress,
			"removing debug configurations in progress")
		return true, nil
	}

	return false, nil
}

// ensureReqMCExists is for ensuring the required machine config exists
func (r *MachineConfigReconciler) ensureReqMCExists(ctx context.Context) (int, error) {
	updatedCount := 0
	if r.CtrlConfig.Spec.Debug.EnableCrioProfiling {
		updated, err := r.enableCrioProf(ctx)
		if err != nil {
			return updatedCount, err
		}
		if updated {
			updatedCount++
		}
	}
	return updatedCount, nil
}

// ensureReqMCNotExists is for ensuring the machine config created when
// profiling was enabled is indeed removed
func (r *MachineConfigReconciler) ensureReqMCNotExists(ctx context.Context) error {
	return r.disableCrioProf(ctx)
}

// ensureReqMCPExists is for ensuring the required machine config pool exists
func (r *MachineConfigReconciler) ensureReqMCPExists(ctx context.Context) (int, error) {
	updatedCount := 0
	updated, err := r.createProfMCP(ctx)
	if err != nil {
		return updatedCount, err
	}
	if updated {
		updatedCount++
	}
	return updatedCount, nil
}

// ensureReqMCPNotExists is for ensuring the machine config pool created when
// profiling was enabled is indeed removed
func (r *MachineConfigReconciler) ensureReqMCPNotExists(ctx context.Context) error {
	return r.deleteProfMCP(ctx)
}

// monitorProgress is for checking the progress of the MCPs based
// on configuration. nodeobservability MCP is checked when debug
// is enabled and worker MCP when disabled
func (r *MachineConfigReconciler) monitorProgress(ctx context.Context) (result ctrl.Result, err error) {

	if r.CtrlConfig.Status.IsDebuggingEnabled() {
		if result, err = r.CheckNodeObservabilityMCPStatus(ctx); err != nil {
			r.Log.Error(err, "failed to check nodeobservability mcp status")
			return
		}
	}

	if !r.CtrlConfig.Status.IsDebuggingEnabled() || r.CtrlConfig.Status.IsDebuggingFailed() {
		if result, err = r.checkWorkerMCPStatus(ctx); err != nil {
			r.Log.Error(err, "failed to check worker mcp status")
			return
		}
	}

	return
}

// revertEnabledProfConf is for restoring the cluster state to
// as it was, before enabling the debug configurations
func (r *MachineConfigReconciler) revertEnabledProfConf(ctx context.Context) error {
	_, err := r.ensureReqNodeLabelNotExists(ctx)
	return err
}
