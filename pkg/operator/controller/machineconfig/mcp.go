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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

// createProfMCP is for creating the MCP required for
// tracking profiling machine configs
func (r *MachineConfigReconciler) createProfMCP(ctx context.Context) (bool, error) {
	createdNow := false
	namespace := types.NamespacedName{Name: ProfilingMCPName}

	err := r.createMCP(ctx, namespace.Name)
	if err != nil && !errors.IsAlreadyExists(err) {
		return createdNow, err
	}
	if err == nil {
		createdNow = true
	}

	// grace time for client cache to get refreshed
	time.Sleep(500 * time.Millisecond)

	if _, err := r.fetchProfMCP(ctx, namespace); err != nil {
		return createdNow, fmt.Errorf("failed to ensure %s MCP was indeed created: %v", namespace.Name, err)
	}

	return createdNow, nil
}

// deleteProfMCP is for removing the MCP created for
// tracking profiling machine configs
func (r *MachineConfigReconciler) deleteProfMCP(ctx context.Context) error {
	namespace := types.NamespacedName{Name: ProfilingMCPName}

	mcp, err := r.fetchProfMCP(ctx, namespace)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if err := r.deleteMCP(ctx, mcp); err != nil {
		return err
	}

	return nil
}

// fetchProfilingMCP is for fetching the profiling MCP created by this controller
func (r *MachineConfigReconciler) fetchProfMCP(ctx context.Context, namespace types.NamespacedName) (*mcv1.MachineConfigPool, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.Get(ctx, namespace, mcp); err != nil {
		return nil, err
	}
	return mcp, nil
}

// getProfilingMCP is for obtaining the tailored profiling MCP
// required for creation
func getProfilingMCP(name string) *mcv1.MachineConfigPool {

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

// createMCP is for creating the required MCP
func (r *MachineConfigReconciler) createMCP(ctx context.Context, name string) error {
	mcp := getProfilingMCP(name)

	if err := r.Create(ctx, mcp); err != nil {
		return fmt.Errorf("failed to create MCP %s: %w", name, err)
	}

	if err := ctrl.SetControllerReference(r.CtrlConfig, mcp, r.Scheme); err != nil {
		r.Log.Error(err, "failed to update owner info in MCP", "MCP", name)
	}

	r.Log.Info("successfully created MCP", "MCP", name)
	return nil
}

// deleteMCP is for deleting MCP passed MCP
func (r *MachineConfigReconciler) deleteMCP(ctx context.Context, mcp *mcv1.MachineConfigPool) error {
	if err := r.Delete(ctx, mcp); err != nil {
		return fmt.Errorf("failed to remove MCP %s : %w", mcp.Name, err)
	}

	r.Log.Info("successfully removed MCP", "MCP", mcp.Name)
	return nil
}

// checkNodeObservabilityMCPStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineConfigReconciler) checkNodeObservabilityMCPStatus(ctx context.Context) (ctrl.Result, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.Get(ctx, types.NamespacedName{Name: ProfilingMCPName}, mcp); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("profiling MCP does not exist, skipping status check", "MCP", ProfilingMCPName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	r.Lock()

	if !v1alpha1.IsNodeObservabilityMachineConfigConditionInProgress(r.CtrlConfig.Status.Conditions, v1alpha1.DebugEnabled) {
		r.Log.Info("NodeObservabilityMachineConfig current condition "+
			"does not require MCP status check",
			"MCP", mcp.Name, "conditions", r.CtrlConfig.Status.Conditions)
		return ctrl.Result{}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) &&
		mcp.Status.DegradedMachineCount == 0 {
		msg := "machine config update to enable debugging in progress"
		r.Log.Info(msg)
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(v1alpha1.DebugEnabled,
			v1alpha1.ConditionInProgress, msg)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)

		r.Unlock()
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "debug config enabled on all machines")
		msg := "machine config update to enable debugging completed on all machines"
		r.Log.Info(msg)

		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(v1alpha1.DebugEnabled, v1alpha1.ConditionTrue, msg)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)

		r.Unlock()
		return ctrl.Result{}, nil
	}

	if mcp.Status.DegradedMachineCount != 0 {
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "ConfigUpdate", "%s MCP has %d machines in degraded state",
			mcp.Name, mcp.Status.DegradedMachineCount)

		r.Unlock()
		if err := r.revertEnabledProfConf(ctx); err != nil {
			r.Lock()
			cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(
				v1alpha1.DebugEnabled,
				v1alpha1.ConditionInProgress,
				fmt.Sprintf("%s MCP has %d machines in degraded state. "+
					"Reverting changes failed, reconcile again",
					mcp.Name, mcp.Status.DegradedMachineCount))
			v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)
			r.Unlock()

			return ctrl.Result{RequeueAfter: defaultRequeueTime},
				fmt.Errorf("failed to revert changes to recover degraded machines: %w", err)
		}

		r.Lock()
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(v1alpha1.Failed,
			v1alpha1.ConditionInProgress,
			fmt.Sprintf("%s MCP has %d machines in degraded state, reverted changes", mcp.Name, mcp.Status.DegradedMachineCount))
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)
		r.Unlock()

		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	r.Unlock()
	r.Log.Info("waiting for machine config update to complete on all machines", "MCP", mcp.Name)
	return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
}

// checkWorkerMCPStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineConfigReconciler) checkWorkerMCPStatus(ctx context.Context) (ctrl.Result, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.Get(ctx, types.NamespacedName{Name: WorkerNodeMCPName}, mcp); err != nil {
		return ctrl.Result{}, err
	}

	r.Lock()

	var condition v1alpha1.NodeObservabilityMachineConfigConditionType
	if v1alpha1.IsNodeObservabilityMachineConfigConditionInProgress(r.CtrlConfig.Status.Conditions, v1alpha1.DebugDisabled) {
		condition = v1alpha1.DebugDisabled
	} else if v1alpha1.IsNodeObservabilityMachineConfigConditionInProgress(r.CtrlConfig.Status.Conditions, v1alpha1.Failed) {
		condition = v1alpha1.Failed
	} else {
		r.Log.Info("NodeObservabilityMachineConfig current condition "+
			"does not require MCP status check",
			"MCP", mcp.Name, "conditions", r.CtrlConfig.Status.Conditions)
		return ctrl.Result{}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) &&
		mcp.Status.DegradedMachineCount == 0 {
		msg := ""
		if condition == v1alpha1.DebugDisabled {
			msg = "machine config update to disable debugging in progress"
		} else {
			msg = "reverting machine config changes due to failure on all machines"
		}
		r.Log.Info(msg)
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(condition,
			v1alpha1.ConditionInProgress, msg)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)

		r.Unlock()
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) {

		r.Unlock()
		if err := r.ensureReqMCNotExists(ctx); err != nil {
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if err := r.ensureReqMCPNotExists(ctx); err != nil {
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}

		msg := ""
		if condition == v1alpha1.DebugDisabled {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "debug config disabled on all machines")
			msg = "machine config update to disable debugging completed on all machines"
		} else {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "debug config reverted on all machines")
			msg = "reverted machine config changes due to failure on all machines"
		}
		r.Log.Info(msg)

		r.Lock()
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(condition, v1alpha1.ConditionTrue, msg)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)
		r.Unlock()

		return ctrl.Result{}, nil
	}

	if mcp.Status.DegradedMachineCount != 0 {
		msg := fmt.Sprintf("%s MCP has %d machines in degraded state", mcp.Name, mcp.Status.DegradedMachineCount)
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "ConfigUpdate", msg)

		if condition == v1alpha1.DebugDisabled {
			msg = fmt.Sprintf("%s, failed to disable debugging, reconcile again", msg)
		} else {
			msg = fmt.Sprintf("%s, failed to revert changes, reconcile again", msg)
		}
		cond := v1alpha1.NewNodeObservabilityMachineConfigCondition(condition, v1alpha1.ConditionInProgress, msg)
		v1alpha1.SetNodeObservabilityMachineConfigCondition(&r.CtrlConfig.Status, *cond)

		r.Unlock()
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
	}

	r.Unlock()
	if condition == v1alpha1.DebugDisabled {
		r.Log.Info("waiting for disabling debugging to complete on all machines", "MCP", mcp.Name)
	} else {
		r.Log.Info("waiting for reverting to complete on all machines", "MCP", mcp.Name)
	}
	return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
}
