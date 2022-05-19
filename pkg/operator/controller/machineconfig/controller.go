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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/go-logr/logr"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch

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
func (r *MachineConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var err error
	if r.Log, err = logr.FromContext(ctx); err != nil {
		return ctrl.Result{}, err
	}

	r.Log.V(3).Info("Reconciling MachineConfig of Nodeobservability operator")

	// Fetch the nodeobservability.olm.openshift.io/machineconfig CR
	r.CtrlConfig = &v1alpha1.NodeObservabilityMachineConfig{}
	if err = r.Get(ctx, req.NamespacedName, r.CtrlConfig); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("MachineConfig resource not found. Ignoring could have been deleted", "name", req.NamespacedName.Name)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "failed to fetch MachineConfig")
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
	}
	r.Log.V(3).Info("MachineConfig resource found")

	if !r.CtrlConfig.DeletionTimestamp.IsZero() {
		r.Log.Info("MachineConfig resource marked for deletetion, cleaning up")
		return r.cleanUp(ctx, req)
	}

	// Set finalizers on the NodeObservability/MachineConfig resource
	updated, err := r.withFinalizers(ctx, req)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update MachineConfig with finalizers: %w", err)
	}
	updated.DeepCopyInto(r.CtrlConfig)

	requeue, err := r.inspectProfilingMCReq(ctx)
	if err != nil {
		r.Log.Error(err, "failed to reconcile requested configuration")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}
	// if the configuration changes were made in the current reconciling
	// will requeue to avoid the existing status of MCP to be considered
	// and allow MCO to pick the changes and update correct state
	if requeue {
		r.Log.Info("updated configurations, reconcile again in 2 minutes")
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	return r.monitorProgress(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodeObservabilityMachineConfig{}).
		Complete(r)
}

func (r *MachineConfigReconciler) cleanUp(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if hasFinalizer(r.CtrlConfig) {
		// Remove the finalizer.
		if _, err := r.withoutFinalizers(ctx, req, finalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from MachineConfig %s: %w", r.CtrlConfig.Name, err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *MachineConfigReconciler) withFinalizers(ctx context.Context, req ctrl.Request) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	withFinalizers := &v1alpha1.NodeObservabilityMachineConfig{}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, req.NamespacedName, withFinalizers); err != nil {
			r.Log.Error(err, "failed to fetch nodeobservabilitymachineconfig resource for updating finalizer")
			return err
		}

		if hasFinalizer(withFinalizers) {
			return nil
		}
		withFinalizers.Finalizers = append(withFinalizers.Finalizers, finalizer)

		if err := r.Update(ctx, withFinalizers); err != nil {
			r.Log.Error(err, "failed to update nodeobservabilitymachineconfig resource finalizers")
			return err
		}

		return nil
	})

	return withFinalizers, err
}

func (r *MachineConfigReconciler) withoutFinalizers(ctx context.Context, req ctrl.Request, finalizer string) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	withoutFinalizers := &v1alpha1.NodeObservabilityMachineConfig{}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, req.NamespacedName, withoutFinalizers); err != nil {
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
		if err := r.Update(ctx, withoutFinalizers); err != nil {
			r.Log.Error(err, "failed to remove nodeobservabilitymachineconfig resource finalizers")
			return err
		}

		return nil
	})

	return withoutFinalizers, err
}

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

func (r *MachineConfigReconciler) updateStatus(ctx context.Context) error {
	if err := r.Status().Update(ctx, r.CtrlConfig); err != nil {
		r.Log.Error(err, "failed to update nodeobservabilitymachineconfig resource status")
		return err
	}
	return nil
}

// inspectProfilingMCReq is for checking and creating required configs
// if debugging is enabled
func (r *MachineConfigReconciler) inspectProfilingMCReq(ctx context.Context) (bool, error) {

	condition := v1alpha1.IsNodeObservabilityMachineConfigConditionSetInProgress(r.CtrlConfig.Status.Conditions)
	if condition != Empty {
		r.Log.Info("previous reconcile initiated operation in progress, changes not applied",
			"condition", condition)
		return false, nil
	}

	if r.CtrlConfig.Spec.Debug != (v1alpha1.NodeObservabilityDebug{}) {
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
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(v1alpha1.DebugEnabled, v1alpha1.ConditionInProgress, Empty)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)
		return true, r.updateStatus(ctx)
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
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(v1alpha1.DebugDisabled, v1alpha1.ConditionInProgress, Empty)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)
		return true, r.updateStatus(ctx)
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
	if !r.CtrlConfig.Spec.Debug.EnableCrioProfiling {
		return r.disableCrioProf(ctx)
	}
	return nil
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

func (r *MachineConfigReconciler) monitorProgress(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {

	if v1alpha1.IsNodeObservabilityMachineConfigConditionInProgress(r.CtrlConfig.Status.Conditions, v1alpha1.DebugEnabled) {
		if result, err = r.CheckNodeObservabilityMCPStatus(ctx); err != nil {
			return
		}
	}

	if v1alpha1.IsNodeObservabilityMachineConfigConditionInProgress(r.CtrlConfig.Status.Conditions, v1alpha1.DebugDisabled) ||
		v1alpha1.IsNodeObservabilityMachineConfigConditionInProgress(r.CtrlConfig.Status.Conditions, v1alpha1.Failed) {
		if result, err = r.checkWorkerMCPStatus(ctx); err != nil {
			return
		}
	}

	if err := r.updateStatus(ctx); err != nil {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	return
}

// revertEnabledProfConf is for restoring the cluster state to
// as it was, before enabling the debug configurations
func (r *MachineConfigReconciler) revertEnabledProfConf(ctx context.Context) error {
	_, err := r.ensureReqNodeLabelNotExists(ctx)
	return err
}
