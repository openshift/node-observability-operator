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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha2"
	"github.com/openshift/node-observability-operator/pkg/util/slice"
)

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create

// MachineConfigReconciler reconciles a NodeObservabilityMachineConfig object
type MachineConfigReconciler struct {
	impl

	Log           logr.Logger
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

// New returns a new MachineConfigReconciler instance.
func New(mgr ctrl.Manager) *MachineConfigReconciler {
	return &MachineConfigReconciler{
		impl:          NewClient(mgr),
		Log:           ctrl.Log.WithName("controller.nodeobservabilitymachineconfig"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetEventRecorderFor("node-observability-operator"),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.NodeObservabilityMachineConfig{}, builder.WithPredicates(ignoreNOMCStatusUpdates())).
		// TODO: add missing watches
		Owns(&mcv1.MachineConfig{}).
		Owns(&mcv1.MachineConfigPool{}).
		Complete(r)
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
func (r *MachineConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if ctxLog, err := logr.FromContext(ctx); err == nil {
		r.Log = ctxLog
	}

	r.Log.V(1).Info("reconciliation started")

	// Fetch the NodeObservabilityMachineConfig CR
	nomc := &v1alpha2.NodeObservabilityMachineConfig{}
	if err := r.ClientGet(ctx, req.NamespacedName, nomc); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.V(1).Info("nodeobservabilitymachineconfig resource not found. Ignoring as it could have been deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to fetch nodeobservabilitymachineconfig %q: %w", req.NamespacedName, err)
	}

	r.Log.V(1).Info("reconciling", "nodeobservabilitymachineconfig", req.NamespacedName)

	if !nomc.DeletionTimestamp.IsZero() {
		r.Log.V(1).Info("marked for deletion, skipping reconciliation", "nodeobservabilitymachineconfig", req.NamespacedName)

		// triggering clean up of machineconfigs, cleanUp() updates
		// nomc status based on state of clean up
		if requeue, err := r.cleanUp(ctx, nomc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to clean up machineconfig due to %w, re-queuing", err)
		} else if requeue {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}

		if err := r.removeFinalizer(ctx, nomc, finalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from nodeobservabilitymachineconfig %q: %w", req.NamespacedName, err)
		}
		r.Log.V(1).Info("removed finalizer, cleanup complete", "nodeobservabilitymachineconfig", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Set finalizers on the NodeObservabilityMachineConfig resource
	nomc, err := r.addFinalizer(ctx, nomc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update nodeobservabilitymachineconfig with finalizers: %w", err)
	}

	if err := r.handleProfilingRequest(ctx, nomc); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile requested configuration: %w", err)
	}

	// adding a deferred update to sync statuses even during errors since we have multiple
	// intermediate status conditions to depict intermediate states of the operator.
	defer func() (ctrl.Result, error) {
		// update nomc status with current status
		if err := r.updateStatus(ctx, nomc); err != nil {
			r.Log.Error(err, "failed to update nodeobservabilitymachineconfig status", "nodeobservabilityconfig.status", nomc.Status)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}() //nolint:errcheck

	// update nomc status based on mcp status conditions
	// after handling profiling request

	// if nodeobservabilitymachineconfig status is DebugEnabled=True
	// we poll until we sync nodeobservability mcp
	// completely
	if nomc.Status.IsDebuggingEnabled() && nomc.Status.IsMachineConfigInProgress() {
		// sync for nodeobservability mcp status
		if requeue, err := r.syncNodeObservabilityMCPStatus(ctx, nomc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to check nodeobservability mcp status: %w", err)
		} else if requeue {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}
	}

	// if nodeobservabilitymachineconfig status is DebugEnabled=False
	// we poll until we sync worker mcp completely
	if !nomc.Status.IsDebuggingEnabled() && nomc.Status.GetCondition(v1alpha2.DebugEnabled).Reason == v1alpha2.ReasonDisabled && nomc.Status.IsMachineConfigInProgress() {
		// sync for worker mcp status
		if requeue, err := r.syncWorkerMCPStatus(ctx, nomc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to check worker mcp status: %w", err)
		} else if requeue {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}
	}

	return ctrl.Result{}, nil
}

// handleProfilingRequest checks the profiling setting and acts accordingly: enable/disable the profiling config.
// Returns true if the requeue is needed.
func (r *MachineConfigReconciler) handleProfilingRequest(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) error {

	if nomc.Spec.Debug.EnableCrioProfiling {
		err := r.ensureProfConfEnabled(ctx, nomc)
		if err != nil {
			r.Log.V(1).Error(err, "failed to enable CRIO profiling, retrying")
			nomc.Status.SetCondition(v1alpha2.DebugEnabled, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "enabling CRIO profiling in progress")
		}
		nomc.Status.SetCondition(v1alpha2.DebugEnabled, metav1.ConditionTrue, v1alpha2.ReasonEnabled, "CRIO debug configurations enabled")
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "enabling debug configurations in progress")
	} else {
		err := r.ensureProfConfDisabled(ctx, nomc)
		if err != nil {
			r.Log.V(1).Error(err, "failed to disable CRI-o profiling, retrying")
			nomc.Status.SetCondition(v1alpha2.DebugEnabled, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "disabling CRIO profiling in progress")
		}
		nomc.Status.SetCondition(v1alpha2.DebugEnabled, metav1.ConditionFalse, v1alpha2.ReasonDisabled, "debug configurations disabled")
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "removing debug configurations in progress")
	}

	return nil
}

// updateStatus is updating the status subresource of NodeObservabilityMachineConfig
func (r *MachineConfigReconciler) updateStatus(ctx context.Context, updated *v1alpha2.NodeObservabilityMachineConfig) error {

	namespace := types.NamespacedName{Name: updated.Name}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.Log.V(1).Info("updating nodeobservabilitymachineconfig status")
		current := &v1alpha2.NodeObservabilityMachineConfig{}
		if err := r.ClientGet(ctx, namespace, current); err != nil {
			return err
		}
		updated.Status.DeepCopyInto(&current.Status)

		if err := r.ClientStatusUpdate(ctx, current); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// cleanUp handles deletion of nodeobservabilitymachineconfig gracefully.
// It initially removes the labels that were added on the nodes and waits
// for the worker mcp to sync the older machineconfig on all worker nodes.
// It returns a true if the clean up is still in progress and the request
// needs re-queuing.
func (r *MachineConfigReconciler) cleanUp(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) (bool, error) {
	err := r.ensureProfConfDisabled(ctx, nomc)
	if err != nil {
		r.Log.V(1).Error(err, "failed to disable CRI-o profiling, retrying")
		nomc.Status.SetCondition(v1alpha2.DebugEnabled, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "disabling CRIO profiling in progress")
		return false, err
	}
	nomc.Status.SetCondition(v1alpha2.DebugEnabled, metav1.ConditionFalse, v1alpha2.ReasonDisabled, "debug configurations disabled")
	nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "removing debug configurations in progress")

	// check worker MCP status polls for worker MCP status
	// and based on this it will trigger a re-queuing of the
	// request.
	requeue, err := r.syncWorkerMCPStatus(ctx, nomc)
	if err != nil {
		r.Log.V(1).Error(err, "failed to sync worker mcp status, retrying")
		return false, err
	}

	// update nomc status with current clean up status
	if err := r.updateStatus(ctx, nomc); err != nil {
		r.Log.Error(err, "failed to update nodeobservabilitymachineconfig status", "status", nomc.Status)
		return false, fmt.Errorf("failed to update nodeobservabilityconfig status due to %w", err)
	}

	// requeue if mcp update still in progress
	if requeue {
		return true, nil
	}

	// since worker mcp is stable, clean up profiling mcp if it exists
	if err := r.deleteCrioProfMC(ctx); err != nil {
		return false, err
	}
	if err := r.deleteProfMCP(ctx); err != nil {
		return false, err
	}

	return false, nil
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

			// if NOMC generated count is unchanged, it indicates
			// spec or metadata has not changed and the event could be for
			// status update which need not be queued for reconciliation
			if _, ok := e.ObjectOld.(*v1alpha2.NodeObservabilityMachineConfig); ok {
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			}
			return true
		},
	}
}

// addFinalizer adds finalizer to NodeObservabilityMachineConfig resource
// if does not exist
func (r *MachineConfigReconciler) addFinalizer(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) (*v1alpha2.NodeObservabilityMachineConfig, error) {
	if !slice.ContainsString(nomc.Finalizers, finalizer) {
		updated := nomc.DeepCopy()
		updated.Finalizers = append(updated.Finalizers, finalizer)

		if err := r.ClientUpdate(ctx, updated); err != nil {
			return nil, fmt.Errorf("failed to add finalizers on %q nomc due to %w", nomc.Name, err)
		}

		if err := r.ClientGet(ctx, types.NamespacedName{Namespace: updated.Namespace, Name: updated.Name}, updated); err != nil {
			return nil, fmt.Errorf("failed to get nodeobservabilitymachineconfig: %w", err)
		}

		return updated, nil

	}

	return nomc, nil
}

// removeFinalizer removes finalizers added to
// NodeObservabilityMachineConfig resource if present
func (r *MachineConfigReconciler) removeFinalizer(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig, finalizer string) error {

	if slice.ContainsString(nomc.Finalizers, finalizer) {
		updated := nomc.DeepCopy()
		updated.Finalizers = slice.RemoveString(updated.Finalizers, finalizer)

		if err := r.ClientUpdate(ctx, updated); err != nil {
			return err
		}
		return nil
	}

	return nil
}
