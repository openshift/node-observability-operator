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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

// createProfMCP creates MachineConfigPool CR to enable the CRI-O profiling on the targeted nodes.
func (r *MachineConfigReconciler) createProfMCP(ctx context.Context) error {
	mcp := r.getCrioProfMachineConfigPool(ProfilingMCPName)

	if err := ctrlutil.SetControllerReference(r.CtrlConfig, mcp, r.Scheme); err != nil {
		return fmt.Errorf("failed to update controller reference in crio profiling machine config pool: %w", err)
	}

	if err := r.ClientCreate(ctx, mcp); err != nil {
		return fmt.Errorf("failed to create crio profiling machine config pool: %w", err)
	}

	r.Log.V(1).Info("Successfully created MachineConfigPool to enable CRI-O profiling", "MCPName", ProfilingMCPName)
	return nil
}

// deleteProfMCP deletes MachineConfigPool CR which enables the CRI-O profiling on the nodes if it exists.
func (r *MachineConfigReconciler) deleteProfMCP(ctx context.Context) error {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: ProfilingMCPName}, mcp); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := r.ClientDelete(ctx, mcp); err != nil {
		return fmt.Errorf("failed to remove crio profiling machine config pool: %w", err)
	}

	r.Log.V(1).Info("Successfully removed MachineConfigPool which was enabling CRI-O profiling", "MCPName", ProfilingMCPName)
	return nil
}

// getCrioProfMachineConfigPool returns the MachineConfigPool CR definition
// to enable CRI-O profiling on the targeted nodes.
func (r *MachineConfigReconciler) getCrioProfMachineConfigPool(name string) *mcv1.MachineConfigPool {

	return &mcv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       MCPoolKind,
			APIVersion: MCAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: MachineConfigLabels,
		},
		Spec: mcv1.MachineConfigPoolSpec{
			MachineConfigSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      MCRoleLabelName,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							WorkerNodeRoleName,
							NodeObservabilityNodeRoleName,
						},
					},
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: NodeSelectorLabels,
			},
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				Source: []corev1.ObjectReference{
					{
						APIVersion: MCAPIVersion,
						Kind:       MCKind,
						Name:       CrioProfilingConfigName,
					},
				},
			},
		},
	}
}

// checkNodeObservabilityMCPStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineConfigReconciler) checkNodeObservabilityMCPStatus(ctx context.Context) (ctrl.Result, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: ProfilingMCPName}, mcp); err != nil {
		if errors.IsNotFound(err) {
			r.Log.V(1).Info("Profiling MCP does not exist, skipping status check", "MCP", ProfilingMCPName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) &&
		mcp.Status.DegradedMachineCount == 0 {
		msg := "Machine config update to enable debugging in progress"
		r.Log.V(1).Info(msg)
		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress, msg)

		return ctrl.Result{}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "debug config enabled on all machines")
		msg := "Machine config update to enable debugging completed on all machines"
		r.Log.V(1).Info(msg)

		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionTrue, v1alpha1.ReasonReady, msg)

		return ctrl.Result{}, nil
	}

	if mcp.Status.DegradedMachineCount != 0 {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "ConfigUpdate", "%s MCP has %d machines in degraded state",
			mcp.Name, mcp.Status.DegradedMachineCount)

		if err := r.revertEnabledProfConf(ctx); err != nil {
			msg := fmt.Sprintf("%s MCP has %d machines in degraded state. Reverting changes failed, reconcile again",
				mcp.Name, mcp.Status.DegradedMachineCount)
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress, msg)

			return ctrl.Result{RequeueAfter: defaultRequeueTime},
				fmt.Errorf("failed to revert changes to recover degraded machines: %w", err)
		}

		msg := fmt.Sprintf("%s MCP has %d machines in degraded state, reverted changes", mcp.Name, mcp.Status.DegradedMachineCount)
		r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, msg)
		return ctrl.Result{}, nil
	}

	r.Log.V(1).Info("Waiting for machine config update to complete on all machines", "MCP", mcp.Name)
	return ctrl.Result{}, nil
}

// checkWorkerMCPStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineConfigReconciler) checkWorkerMCPStatus(ctx context.Context) (ctrl.Result, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: WorkerNodeMCPName}, mcp); err != nil {
		return ctrl.Result{}, err
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) &&
		mcp.Status.DegradedMachineCount == 0 {
		var msg string
		if !r.CtrlConfig.Status.IsDebuggingEnabled() {
			msg = "Machine config update to disable debugging in progress"
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress, msg)
		}
		if r.CtrlConfig.Status.IsDebuggingFailed() {
			msg = "Reverting machine config changes due to failure on all machines"
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress, msg)
		}
		r.Log.V(1).Info(msg)
		return ctrl.Result{}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) {

		if err := r.ensureReqMCNotExists(ctx); err != nil {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
		}
		if err := r.ensureReqMCPNotExists(ctx); err != nil {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, err
		}

		var msg string
		if !r.CtrlConfig.Status.IsDebuggingEnabled() {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "debug config disabled on all machines")
			msg = "Machine config update to disable debugging completed on all machines"
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonDisabled, msg)
		}
		if r.CtrlConfig.Status.IsDebuggingFailed() {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "debug config reverted on all machines")
			msg = "Reverted machine config changes due to failure on all machines"
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, msg)
		}

		r.Log.V(1).Info(msg)

		return ctrl.Result{}, nil
	}

	if mcp.Status.DegradedMachineCount != 0 {
		msg := fmt.Sprintf("%s MCP has %d machines in degraded state", mcp.Name, mcp.Status.DegradedMachineCount)
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "ConfigUpdate", msg)

		if !r.CtrlConfig.Status.IsDebuggingEnabled() {
			msg = fmt.Sprintf("%s, failed to disable debugging, reconcile again", msg)
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress, msg)
		}
		if r.CtrlConfig.Status.IsDebuggingFailed() {
			msg = fmt.Sprintf("%s, failed to revert changes, reconcile again", msg)
			r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady, metav1.ConditionFalse, v1alpha1.ReasonInProgress, msg)
		}

		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	if !r.CtrlConfig.Status.IsDebuggingEnabled() {
		r.Log.V(1).Info("Waiting for disabling debugging to complete on all machines", "MCP", mcp.Name)
	}
	if r.CtrlConfig.Status.IsDebuggingFailed() {
		r.Log.V(1).Info("Waiting for reverting to complete on all machines", "MCP", mcp.Name)
	}
	return ctrl.Result{}, nil
}
