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
)

// ensureProfilingMCPExists is for creating the MCP required for
// tracking profiling machine configs
func (r *MachineConfigReconciler) ensureProfilingMCPExists(ctx context.Context) (*mcv1.MachineConfigPool, error) {

	namespace := types.NamespacedName{Name: ProfilingMCPName}

	mcp, exists, err := r.fetchProfilingMCP(ctx, namespace)
	if err != nil {
		return nil, err
	}

	if (r.CtrlConfig.Spec.EnableCrioProfiling || r.CtrlConfig.Spec.EnableKubeletProfiling) && !exists {
		if err := r.createProfilingMCP(ctx); err != nil {
			return nil, err
		}

		mcp, found, err := r.fetchProfilingMCP(ctx, namespace)
		if err != nil || !found {
			return nil, fmt.Errorf("failed to fetch just created profiling MCP: %v", err)
		}

		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "CreateConfig", "successfully created %s mcp", ProfilingMCPName)
		return mcp, nil
	}

	if !r.CtrlConfig.Spec.EnableCrioProfiling && !r.CtrlConfig.Spec.EnableKubeletProfiling && exists {
		if err := r.deleteProfilingMCP(ctx, mcp); err != nil {
			return nil, err
		}
		r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "DeleteConfig", "successfully deleted %s mcp", ProfilingMCPName)
		return nil, nil
	}

	return mcp, nil
}

// fetchProfilingMCP is for fetching the profiling MCP created by this controller
func (r *MachineConfigReconciler) fetchProfilingMCP(ctx context.Context, namespace types.NamespacedName) (*mcv1.MachineConfigPool, bool, error) {
	mcp := &mcv1.MachineConfigPool{}

	if err := r.Get(ctx, namespace, mcp); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return mcp, true, nil
}

// getProfilingMCP is for obtaining the tailored profiling MCP
// required for creation
func getProfilingMCP() *mcv1.MachineConfigPool {

	return &mcv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       MCPoolKind,
			APIVersion: MCAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   ProfilingMCPName,
			Labels: MachineConfigLabels,
		},
		Spec: mcv1.MachineConfigPoolSpec{
			MachineConfigSelector: &metav1.LabelSelector{
				MatchLabels: ProfilingMCSelectorLabels,
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
					{
						APIVersion: MCAPIVersion,
						Kind:       MCKind,
						Name:       KubeletGeneratedConfigName,
					},
				},
			},
		},
	}
}

// createProfilingMCP is for creating the required profiling MCP
func (r *MachineConfigReconciler) createProfilingMCP(ctx context.Context) error {
	mcp := getProfilingMCP()

	if err := r.Create(ctx, mcp); err != nil {
		return fmt.Errorf("failed to create MCP for profiling machine configs: %w", err)
	}

	if err := ctrl.SetControllerReference(r.CtrlConfig, mcp, r.Scheme); err != nil {
		r.Log.Error(err, "failed to update owner info in profiling MCP resource")
	}

	r.Log.Info("successfully created MCP for profiling machine configs", "ProfilingMCPName", ProfilingMCPName)
	return nil
}

// deleteProfilingMCP is for deleting MCP created for profiling MC CRs
func (r *MachineConfigReconciler) deleteProfilingMCP(ctx context.Context, mcp *mcv1.MachineConfigPool) error {
	if err := r.Delete(ctx, mcp); err != nil {
		return fmt.Errorf("failed to remove profiling MCP CR %s : %w", ProfilingMCPName, err)
	}

	r.Log.Info("successfully removed profiling MCP CR", "ProfilingMCPName", ProfilingMCPName)
	return nil
}

// checkMCPUpdateStatus is for reconciling update status of all machines in profiling MCP
func (r *MachineConfigReconciler) checkMCPUpdateStatus(ctx context.Context) (ctrl.Result, error) {
	mcp := &mcv1.MachineConfigPool{}
	key := types.NamespacedName{Name: ProfilingMCPName}
	if err := r.Get(ctx, key, mcp); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("profiling MCP does not exist, skipping status check", "ProfilingMCPName", ProfilingMCPName)
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}
		return ctrl.Result{}, err
	}

	r.Lock()
	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdating) &&
		r.CtrlConfig.Status.UpdateStatus.InProgress == corev1.ConditionFalse {
		r.Log.Info("machine config update under progress")
		r.CtrlConfig.Status.UpdateStatus.InProgress = corev1.ConditionTrue
		r.Unlock()
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	if mcv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcv1.MachineConfigPoolUpdated) {
		if mcp.Status.UpdatedMachineCount == mcp.Status.MachineCount {
			r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeNormal, "ConfigUpdate", "config update completed on all machines")
			r.Log.Info("machine config update completed on all machines")
			r.CtrlConfig.Status.UpdateStatus.InProgress = corev1.ConditionFalse
			r.Unlock()
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}
		if mcp.Status.UpdatedMachineCount != mcp.Status.MachineCount {
			if mcp.Status.DegradedMachineCount != 0 {
				r.EventRecorder.Eventf(r.CtrlConfig, corev1.EventTypeWarning, "ConfigUpdate", "%s MCP has %d machines in degraded state",
					ProfilingMCPName, mcp.Status.DegradedMachineCount)

				r.Unlock()
				if err := r.revertPrevSyncChanges(ctx); err != nil {
					return ctrl.Result{RequeueAfter: 3 * time.Minute},
						fmt.Errorf("failed to revert changes to recover degraded machines")
				}
				return ctrl.Result{RequeueAfter: 3 * time.Minute},
					fmt.Errorf("%d machines are in degraded state", mcp.Status.DegradedMachineCount)
			}
			r.CtrlConfig.Status.UpdateStatus.InProgress = corev1.ConditionTrue
			r.Unlock()
			r.Log.Info("waiting for machine config update to complete on all machines", "MachineConfigPool", ProfilingMCPName)
			return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
		}
	}
	r.Unlock()
	return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
}
