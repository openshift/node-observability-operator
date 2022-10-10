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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha2"
)

// createProfMCP creates MachineConfigPool CR to enable the CRI-O profiling on the targeted nodes.
func (r *MachineConfigReconciler) createProfMCP(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) error {
	mcp := r.getCrioProfMachineConfigPool(ProfilingMCPName)

	if err := ctrlutil.SetControllerReference(nomc, mcp, r.Scheme); err != nil {
		return fmt.Errorf("failed to update controller reference in crio profiling machine config pool: %w", err)
	}

	if err := r.ClientCreate(ctx, mcp); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create crio profiling machine config pool: %w", err)
	}

	// TODO: check if there is a diff between desired and exists

	r.Log.V(1).Info("Successfully created MachineConfigPool to enable CRI-O profiling", "MCPName", ProfilingMCPName)
	return nil
}

// deleteProfMCP deletes MachineConfigPool CR which enables the CRI-O profiling on the nodes if it exists.
func (r *MachineConfigReconciler) deleteProfMCP(ctx context.Context) error {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: ProfilingMCPName}, mcp); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := r.ClientDelete(ctx, mcp); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to remove crio profiling machineconfigpool: %w", err)
	}

	r.Log.V(1).Info("successfully removed machineconfigpool which was enabling CRI-O profiling", "MCPName", ProfilingMCPName)
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
						Name:       crioProfilingConfigName,
					},
				},
			},
		},
	}
}

// syncNodeObservabilityMCPStatus is for reconciling update status
// of all machines in profiling MCP. It returns a boolean indicating
// if the request needs to be requeued. It also evaluates the status
// on the associated nodeobservabilitymachineconfig.
func (r *MachineConfigReconciler) syncNodeObservabilityMCPStatus(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) (bool, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: ProfilingMCPName}, mcp); err != nil {
		return false, err
	}

	r.Log.V(1).Info("checking current status of", "mcp", mcp.Name)

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) && mcp.Status.DegradedMachineCount == 0 {
		r.Log.V(1).Info("nodeobservability mcp", "status", mcv1.MachineConfigPoolUpdating)
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "machineconfig update to enable debugging in progress")
		return true, nil
	} else if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) && r.hasRequiredMachineCount(ctx, mcp) && mcp.Status.DegradedMachineCount == 0 {
		r.EventRecorder.Eventf(nomc, corev1.EventTypeNormal, "ConfigUpdate", "debug config enabled on all machines")
		r.Log.V(1).Info("nodeobservability mcp", "status", mcv1.MachineConfigPoolUpdated)
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionTrue, v1alpha2.ReasonReady, "machineconfig update to enable debugging completed on all machines")
		return false, nil
	} else if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolDegraded) && mcp.Status.DegradedMachineCount != 0 {
		msg := fmt.Sprintf("%s MCP has %d machines in degraded state", mcp.Name, mcp.Status.DegradedMachineCount)
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonFailed, msg)
		return false, fmt.Errorf("failed to update machineconfig on %s mcp due to degraded machines: %d", mcp.Name, mcp.Status.DegradedMachineCount)
	}

	r.Log.V(1).Info("waiting for nodeobservability mcp to complete updating on all machines")
	return true, nil
}

// syncWorkerMCPStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineConfigReconciler) syncWorkerMCPStatus(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) (bool, error) {
	mcp := &mcv1.MachineConfigPool{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: WorkerNodeMCPName}, mcp); err != nil {
		return false, err
	}

	r.Log.V(1).Info("checking current status of", "mcp", mcp.Name)

	// if worker mcp is still updating, update nomc status to in progress and requeue.
	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) && mcp.Status.DegradedMachineCount == 0 {
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonInProgress, "machineconfig update to disable debugging in progress")
		r.Log.V(1).Info("updating nodeobservabilitymachineconfig status to in progress", "status", nomc.Status)
		return true, nil
	} else if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) && r.hasRequiredMachineCount(ctx, mcp) && mcp.Status.DegradedMachineCount == 0 {
		r.EventRecorder.Eventf(nomc, corev1.EventTypeNormal, "ConfigUpdate", "debug config disabled on all machines")
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonDisabled, "machineconfig update to disable debugging completed on all machines")
		r.Log.V(1).Info("updating nodeobservabilitymachineconfig status to completed disabling profiling", "status", nomc.Status)
		return false, nil
	} else if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolDegraded) && mcp.Status.DegradedMachineCount != 0 {
		msg := fmt.Sprintf("failed to disable debugging due to %s mcp has %d machines in degraded state, reconcile again", mcp.Name, mcp.Status.DegradedMachineCount)
		r.EventRecorder.Eventf(nomc, corev1.EventTypeWarning, "ConfigUpdate", msg)
		nomc.Status.SetCondition(v1alpha2.DebugReady, metav1.ConditionFalse, v1alpha2.ReasonInProgress, msg)
		return true, nil
	}

	// trigger requeue since still waiting for update
	r.Log.V(1).Info("Waiting for disabling debugging to complete on all machines", "MCP", mcp.Name)
	return true, nil
}

func (r *MachineConfigReconciler) hasRequiredMachineCount(ctx context.Context, mcp *mcv1.MachineConfigPool) bool {
	labels := mcp.Spec.NodeSelector.MatchLabels
	nodeList, err := r.listNodes(ctx, labels)
	if err != nil {
		r.Log.Error(err, "failed to list nodes with", "matchlabels", labels)
		return false
	}

	requiredNodeCount := len(nodeList.Items)
	r.Log.V(2).Info(fmt.Sprintf("required nodes: %d", requiredNodeCount), "mcp.Status", mcp.Status)

	return mcp.Status.MachineCount == int32(requiredNodeCount) &&
		mcp.Status.UpdatedMachineCount == int32(requiredNodeCount) &&
		mcp.Status.ReadyMachineCount == int32(requiredNodeCount)
}
