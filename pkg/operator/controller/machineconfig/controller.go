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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

// MachineconfigReconciler reconciles a Machineconfig object
type MachineconfigReconciler struct {
	client.Client

	Scheme         *runtime.Scheme
	Log            logr.Logger
	CtrlConfig     *v1alpha1.Machineconfig
	EventRecorder  record.EventRecorder
	PrevSyncChange map[string]PrevSyncData
}

type PrevSyncData struct {
	action string
	config interface{}
}

const (
	// MCAPIVersion is the machine config API version
	MCAPIVersion = "machineconfiguration.openshift.io/v1"

	// MCKind is the machine config resource kind
	MCKind = "MachineConfig"

	// MCPoolKind is the machine config pool resource king
	MCPoolKind = "MachineConfigPool"

	// ProfilingMCPName is the name of MCP created for
	// CRI-O, Kubelet... machine configs by this controller
	ProfilingMCPName = "profiling"
)

var (
	// ProfilingMCSelectorLabels is for storing the labels to
	// match with profiling MCP
	ProfilingMCSelectorLabels = map[string]string{
		"machineconfiguration.openshift.io/role": ProfilingMCPName,
	}
)

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=machineconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=machineconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=machineconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Machineconfig object Gagainst the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *MachineconfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling MachineConfig of Nodeobservability operator")

	// Fetch the nodeobservability.olm.openshift.io/machineconfig CR
	r.CtrlConfig = &v1alpha1.Machineconfig{}
	err := r.Get(ctx, req.NamespacedName, r.CtrlConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("MachineConfig resource not found. Ignoring could have been deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "failed to fetch MachineConfig")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}
	r.Log.Info("MachineConfig resource found", "namespace", req.NamespacedName.Namespace, "name", req.NamespacedName.Name)

	if _, err := r.ensureProfilingMCPExists(ctx); err != nil {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "CreateConfigFailed", "failed to create %s mcp", ProfilingMCPName)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// ensure profiling config conform with the spec properties
	if err := r.checkProfConf(ctx); err != nil {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "CreateConfigFailed", "failed to create profiling configs", ProfilingMCPName)
		r.Log.Error(err, "reconciling")
	}

	return r.checkMCPUpdateStatus(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineconfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Machineconfig{}).
		Complete(r)
}

func (r *MachineconfigReconciler) ensureProfilingMCPExists(ctx context.Context) (*mcv1.MachineConfigPool, error) {

	namespace := types.NamespacedName{Namespace: r.CtrlConfig.Namespace, Name: ProfilingMCPName}

	mcp, exist, err := r.fetchProfilingMCP(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if !exist {
		if err := r.createProfilingMCP(ctx); err != nil {
			return nil, err
		}

		mcp, exists, err := r.fetchProfilingMCP(ctx, namespace)
		if err != nil || !exists {
			return nil, fmt.Errorf("failed to fetch just created profiling MCP: %w", err)
		}

		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "CreateConfig", "successfully created %s mcp", ProfilingMCPName)
		return mcp, nil
	}
	return mcp, nil
}

func (r *MachineconfigReconciler) fetchProfilingMCP(ctx context.Context, namespace types.NamespacedName) (*mcv1.MachineConfigPool, bool, error) {
	mcp := &mcv1.MachineConfigPool{}

	if err := r.Get(ctx, namespace, mcp); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return mcp, true, nil
}

func getProfilingMCP() *mcv1.MachineConfigPool {
	nodeSelectorLabels := map[string]string{
		"node-role.kubernetes.io/worker": "",
	}

	return &mcv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       MCPoolKind,
			APIVersion: MCAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ProfilingMCPName,
		},
		Spec: mcv1.MachineConfigPoolSpec{
			MachineConfigSelector: &metav1.LabelSelector{
				MatchLabels: ProfilingMCSelectorLabels,
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: nodeSelectorLabels,
			},
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				Source: []corev1.ObjectReference{
					{
						APIVersion: MCAPIVersion,
						Kind:       MCKind,
						Name:       CrioProfilingConfigName,
					},
					{
						APIVersion: MCAPIVersion,
						Kind:       MCKind,
						Name:       KubeletProfilingConfigName,
					},
				},
			},
		},
	}
}

func (r *MachineconfigReconciler) createProfilingMCP(ctx context.Context) error {
	mcp := getProfilingMCP()

	if err := r.Create(ctx, mcp); err != nil {
		return fmt.Errorf("failed to create MCP for profiling machine configs: %w", err)
	}

	ctrl.SetControllerReference(r.CtrlConfig, mcp, r.Scheme)
	r.Log.Info("successfully created MCP(%s) for profiling machine configs", ProfilingMCPName)
	return nil
}

// checkProfConf checks and ensures profiling config for defined services
func (r *MachineconfigReconciler) checkProfConf(ctx context.Context) error {

	errored := false
	errs := fmt.Errorf("failed to check profiling configs")
	if r.CtrlConfig.Spec.EnableCrioProfiling {
		criomc, created, err := r.ensureCrioProfConfigExists(ctx)
		if err != nil {
			errored = true
			r.Log.Error(err, "failed to enable crio profiling")
			errs = fmt.Errorf("%w: %s", errs, err.Error())
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
			errored = true
			r.Log.Error(err, "failed to disable crio profiling")
			errs = fmt.Errorf("%w: %s", errs, err.Error())
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
			errored = true
			r.Log.Error(err, "failed to enable kubelet profiling")
			errs = fmt.Errorf("%w: %s", errs, err.Error())
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
			errored = true
			r.Log.Error(err, "failed to disable kubelet profiling")
			errs = fmt.Errorf("%w: %s", errs, err.Error())
		}
		if deleted {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "DeleteConfig", "successfully deleted kubelet config")
			r.PrevSyncChange["kubelet"] = PrevSyncData{
				action: "deleted",
			}
		}
	}

	if errored {
		return errs
	}
	return nil
}

// checkMCPUpdateStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineconfigReconciler) checkMCPUpdateStatus(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	mcp := &mcv1.MachineConfigPool{}
	key := types.NamespacedName{Namespace: r.CtrlConfig.Namespace, Name: ProfilingMCPName}
	if err := r.Client.Get(ctx, key, mcp); err != nil {
		r.Log.Error(err, "failed to fetch profiling MCP: %v", err)
		return ctrl.Result{}, err
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) &&
		r.CtrlConfig.Status.UpdateStatus.InProgress == "false" {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "config update still under progress")
		r.Log.Info("config update under progress")
		r.CtrlConfig.Status.UpdateStatus.InProgress = corev1.ConditionTrue
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) &&
		mcp.Status.UpdatedMachineCount == mcp.Status.MachineCount {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "config update completed on all machines")
		r.Log.Info("config update completed on all machines")
		r.CtrlConfig.Status.UpdateStatus.InProgress = "false"
	} else {
		if mcp.Status.DegradedMachineCount != 0 {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "ConfigUpdate", "%s MCP has %d machines in degraded state",
				ProfilingMCPName, mcp.Status.DegradedMachineCount)

			if err := r.revertPrevSyncChanges(ctx); err != nil {
				return ctrl.Result{RequeueAfter: 15 * time.Second},
					fmt.Errorf("failed to revert changes to recover degraded machines, will reconcile in 15s")
			}
			return ctrl.Result{RequeueAfter: 15 * time.Second},
				fmt.Errorf("%d machines are in degraded state, will reconcile in 15s", mcp.Status.DegradedMachineCount)
		}
		r.Log.Info("waiting for update to finish on all machines", "MachineConfigPool", ProfilingMCPName)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MachineconfigReconciler) revertPrevSyncChanges(ctx context.Context) error {
	if len(r.PrevSyncChange) == 0 {
		r.Log.Info("%s MCP has machines in degarded state, no changes made by the controller")
		return nil
	}

	for srv, sync := range r.PrevSyncChange {
		if srv == "crio" {
			if sync.action == "created" {
				criomc, ok := sync.config.(mcv1.MachineConfig)
				if ok {
					return r.deleteCrioProfileConfig(ctx, &criomc)
				}
			}
			if sync.action == "deleted" {
				return r.createCrioProfileConfig(ctx)
			}
		}
		if srv == "kubelet" {
			if sync.action == "created" {
				kubeletmc, ok := sync.config.(mcv1.KubeletConfig)
				if ok {
					return r.deleteKubeletProfileConfig(ctx, &kubeletmc)
				}
			}
			if sync.action == "deleted" {
				return r.createKubeletProfileConfig(ctx)
			}
		}
	}

	return nil
}
