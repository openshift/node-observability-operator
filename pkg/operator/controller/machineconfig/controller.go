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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

// MachineConfigReconciler reconciles a NodeObservabilityMachineConfig object
type MachineConfigReconciler struct {
	client.Client
	sync.RWMutex

	Scheme         *runtime.Scheme
	Log            logr.Logger
	CtrlConfig     *v1alpha1.NodeObservabilityMachineConfig
	EventRecorder  record.EventRecorder
	PrevSyncChange map[string]PrevSyncData
}

// PrevSyncData is for storing the config changes made in
// previous reconciliation and the config used.
type PrevSyncData struct {
	action string
	config interface{}
}

const (
	finalizer = "NodeObservabilityMachineConfig"

	defaultRequeueTime = 30 * time.Minute

	// MCAPIVersion is the machine config API version
	MCAPIVersion = "machineconfiguration.openshift.io/v1"

	// MCKind is the machine config resource kind
	MCKind = "MachineConfig"

	// MCPoolKind is the machine config pool resource king
	MCPoolKind = "MachineConfigPool"

	// ProfilingMCPName is the name of MCP created for
	// CRI-O, Kubelet... machine configs by this controller
	ProfilingMCPName = "nodeobservability"
)

var (
	// ProfilingMCSelectorLabels is for storing the labels to
	// match with profiling MCP
	ProfilingMCSelectorLabels = map[string]string{
		"machineconfiguration.openshift.io/role": ProfilingMCPName,
	}

	// NodeSelectorLabels is for storing the labels to
	// match the nodes to include in MCP
	NodeSelectorLabels = map[string]string{
		"node-role.kubernetes.io/worker": "",
	}

	// MachineConfigLabels is for storing the labels to
	// add in machine config resources
	MachineConfigLabels = map[string]string{
		"machineconfiguration.openshift.io/role":                      ProfilingMCPName,
		"machineconfigs.nodeobservability.olm.openshift.io/profiling": "",
	}
)

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilitymachineconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=kubeletconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *MachineConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {

	if r.Log, err = logr.FromContext(ctx); err != nil {
		return
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
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "failed to fetch MachineConfig")
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, err
	}
	r.Log.V(3).Info("MachineConfig resource found")

	if r.CtrlConfig.DeletionTimestamp != nil {
		r.Log.Info("MachineConfig resource marked for deletetion, cleaning up")
		return r.cleanUp(ctx)
	}

	// Set finalizers on the NodeObservability/MachineConfig resource
	updated, err := r.withFinalizers(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update MachineConfig with finalizers: %w", err)
	}
	r.CtrlConfig = updated

	if _, err := r.ensureProfilingMCPExists(ctx); err != nil {
		r.Log.Error(err, "profiling mcp reconciliation")
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
	}

	// ensure profiling config conform with the spec properties
	if err := r.checkProfConf(ctx); err != nil {
		r.Log.Error(err, "profiling mc reconciliation")
	}

	if result, err := r.CheckMCPUpdateStatus(ctx); err != nil {
		return result, err
	}

	now := metav1.Now()
	r.CtrlConfig.Status.LastUpdated = &now
	if err = r.Status().Update(ctx, r.CtrlConfig); err != nil {
		r.Log.Error(err, "failed to update nodeobservabilitymachineconfig status")
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
	}

	return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodeObservabilityMachineConfig{}).
		Complete(r)
}

func (r *MachineConfigReconciler) cleanUp(ctx context.Context) (ctrl.Result, error) {
	if hasFinalizer(r.CtrlConfig) {
		// Remove the finalizer.
		if _, err := r.withoutFinalizers(ctx, finalizer); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from MachineConfig %s: %w", r.CtrlConfig.Name, err)
		}
	}
	return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
}

func (r *MachineConfigReconciler) withFinalizers(ctx context.Context) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	withFinalizers := r.CtrlConfig.DeepCopy()

	if !hasFinalizer(withFinalizers) {
		withFinalizers.Finalizers = append(withFinalizers.Finalizers, finalizer)
	}

	if err := r.Update(ctx, withFinalizers); err != nil {
		return withFinalizers, fmt.Errorf("failed to update finalizers: %w", err)
	}
	return withFinalizers, nil
}

func (r *MachineConfigReconciler) withoutFinalizers(ctx context.Context, finalizer string) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	withoutFinalizers := r.CtrlConfig.DeepCopy()

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
		return withoutFinalizers, err
	}
	return withoutFinalizers, nil
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

// checkProfConf checks and ensures profiling config for defined services
func (r *MachineConfigReconciler) checkProfConf(ctx context.Context) error {
	r.Lock()
	defer r.Unlock()

	var errors []error
	if r.CtrlConfig.Spec.EnableCrioProfiling {
		criomc, created, err := r.ensureCrioProfConfigExists(ctx)
		if err != nil {
			r.Log.Error(err, "failed to enable crio profiling")
			errors = append(errors, err)
		}
		if created {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "CreateConfig", "successfully created crio machine config")
			r.PrevSyncChange["crio"] = PrevSyncData{
				action: "created",
				config: *criomc,
			}
		}
	} else {
		deleted, err := r.ensureCrioProfConfigNotExists(ctx)
		if err != nil {
			r.Log.Error(err, "failed to disable crio profiling")
			errors = append(errors, err)
		}
		if deleted {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "DeleteConfig", "successfully deleted crio machine config")
			r.PrevSyncChange["crio"] = PrevSyncData{
				action: "deleted",
			}
		}
	}

	if r.CtrlConfig.Spec.EnableKubeletProfiling {
		kubeletmc, created, err := r.ensureKubeletProfConfigExists(ctx)
		if err != nil {
			r.Log.Error(err, "failed to enable kubelet profiling")
			errors = append(errors, err)
		}
		if created {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "CreateConfig", "successfully created kubelet config")
			r.PrevSyncChange["kubelet"] = PrevSyncData{
				action: "created",
				config: *kubeletmc,
			}
		}
	} else {
		deleted, err := r.ensureKubeletProfConfigNotExists(ctx)
		if err != nil {
			r.Log.Error(err, "failed to disable kubelet profiling")
			errors = append(errors, err)
		}
		if deleted {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "DeleteConfig", "successfully deleted kubelet config")
			r.PrevSyncChange["kubelet"] = PrevSyncData{
				action: "deleted",
			}
		}
	}

	return utilerrors.NewAggregate(errors)
}

// revertPrevSyncChanges is for restoring the cluster state to
// as it was, before the changes made in previous reconciliation if any
func (r *MachineConfigReconciler) revertPrevSyncChanges(ctx context.Context) error {
	r.Lock()
	defer r.Unlock()

	if len(r.PrevSyncChange) == 0 {
		r.Log.Info("profiling MCP has machines in degraded state, not because of any changes made by this controller")
		return nil
	}

	if psd, ok := r.PrevSyncChange["crio"]; ok {
		var err error
		if psd.action == "created" {
			criomc, ok := psd.config.(mcv1.MachineConfig)
			if ok {
				err = r.deleteCrioProfileConfig(ctx, &criomc)
			}
		}
		if psd.action == "deleted" {
			err = r.createCrioProfileConfig(ctx)
		}
		if err == nil {
			delete(r.PrevSyncChange, "crio")
		}
		return err
	}

	if psd, ok := r.PrevSyncChange["kubelet"]; ok {
		var err error
		if psd.action == "created" {
			kubeletmc, ok := psd.config.(mcv1.KubeletConfig)
			if ok {
				err = r.deleteKubeletProfileConfig(ctx, &kubeletmc)
			}
		}
		if psd.action == "deleted" {
			err = r.createKubeletProfileConfig(ctx)
		}
		if err == nil {
			delete(r.PrevSyncChange, "kubelet")
		}
		return err
	}

	return nil
}
