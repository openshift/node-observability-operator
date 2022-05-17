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

package nodeobservabilityruncontroller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	MCOKind       = "NodeObservabilityMachineConfig"
	MCOApiVersion = "nodeobservability.olm.openshift.io/v1alpha1"
	MCOName       = "nodeobservabilitymachineconfig-run"
)

// ensureMCO ensures that the node observability machineconfig is created
// Returns a Boolean value indicating whether it exists, a pointer to the
// daemonset and an error when relevant
func (r *NodeObservabilityRunReconciler) ensureMCO(ctx context.Context) (bool, *v1alpha1.NodeObservabilityMachineConfig, error) {
	desired := r.desiredMCO()
	nameSpace := types.NamespacedName{Namespace: "", Name: desired.Name}
	exist, current, err := r.currentMCO(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get NodeObservabilityMachineConfig: %v", err)
	}
	if !exist {
		if err := r.createMCO(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentMCO(ctx, nameSpace)
	}
	return true, current, err
}

// currentMCO check if the node observability machineconfig exists
func (r *NodeObservabilityRunReconciler) currentMCO(ctx context.Context, nameSpace types.NamespacedName) (bool, *v1alpha1.NodeObservabilityMachineConfig, error) {
	mc := r.desiredMCO()
	if err := r.Get(ctx, nameSpace, mc); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, mc, nil
}

// createMachineConfigt creates the node observability machineconfig
func (r *NodeObservabilityRunReconciler) createMCO(ctx context.Context, mc *v1alpha1.NodeObservabilityMachineConfig) error {
	if err := r.Create(ctx, mc); err != nil {
		return fmt.Errorf("failed to create MachineConfig %s: %w", mc.Name, err)
	}
	r.Log.Info("created MachinConfig", "MachineConfig.Name", mc.Name)
	return nil
}

// desiredDaemonSet returns a DaemonSet object
func (r *NodeObservabilityRunReconciler) desiredMCO() *v1alpha1.NodeObservabilityMachineConfig {
	return &v1alpha1.NodeObservabilityMachineConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       MCOKind,
			APIVersion: MCOApiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: MCOName,
		},
		Spec: v1alpha1.NodeObservabilityMachineConfigSpec{
			Debug: v1alpha1.NodeObservabilityDebug{
				EnableCrioProfiling: true,
			},
		},
	}
}

func (r *NodeObservabilityRunReconciler) ensureNOMC(ctx context.Context) error {
	desired := r.desiredMCO()
	nameSpace := types.NamespacedName{Name: desired.Name}
	err := r.createNOMC(ctx, desired)
	if errors.IsAlreadyExists(err) {
		nomc, err := r.fetchNOMC(ctx, nameSpace)
		if err != nil {
			return err
		}
		if !nomc.Spec.Debug.EnableCrioProfiling {
			nomc.Spec.Debug.EnableCrioProfiling = true
			return r.updateNOMC(ctx, nomc)
		}
		return nil
	}
	return err
}

func (r *NodeObservabilityRunReconciler) fetchNOMC(ctx context.Context, nameSpace types.NamespacedName) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	mc := &v1alpha1.NodeObservabilityMachineConfig{}
	if err := r.Get(ctx, nameSpace, mc); err != nil {
		return nil, fmt.Errorf("failed to fetch NodeObservabilityMachineConfig %s: %w", mc.Name, err)
	}
	r.Log.V(3).Info("fetched NodeObservabilityMachinConfig", "name", mc.Name)
	return mc, nil
}

func (r *NodeObservabilityRunReconciler) createNOMC(ctx context.Context, mc *v1alpha1.NodeObservabilityMachineConfig) error {
	if err := r.Create(ctx, mc); err != nil {
		return fmt.Errorf("failed to create NodeObservabilityMachineConfig %s: %w", mc.Name, err)
	}
	r.Log.Info("created NodeObservabilityMachinConfig", "name", mc.Name)
	return nil
}

func (r *NodeObservabilityRunReconciler) updateNOMC(ctx context.Context, mc *v1alpha1.NodeObservabilityMachineConfig) error {
	if err := r.Update(ctx, mc); err != nil {
		return fmt.Errorf("failed to update NodeObservabilityMachineConfig %s: %w", mc.Name, err)
	}
	r.Log.Info("updated NodeObservabilityMachinConfig", "name", mc.Name)
	return nil
}

func (r *NodeObservabilityRunReconciler) deleteNOMC(ctx context.Context, mc *v1alpha1.NodeObservabilityMachineConfig) error {
	if err := r.Delete(ctx, mc); err != nil {
		return fmt.Errorf("failed to delete NodeObservabilityMachineConfig %s: %w", mc.Name, err)
	}
	r.Log.Info("deleted NodeObservabilityMachinConfig", "name", mc.Name)
	return nil
}

func (r *NodeObservabilityRunReconciler) checkNOMCStatus(ctx context.Context, condType v1alpha1.NodeObservabilityMachineConfigConditionType) (bool, error) {
	nameSpace := types.NamespacedName{Name: MCOName}
	nomc, err := r.fetchNOMC(ctx, nameSpace)
	if err != nil {
		return false, err
	}

	if len(nomc.Status.Conditions) == 0 {
		r.Log.Info("NodeObservabilityMachineConfig has not updated any conditions yet")
		return false, nil
	}

	for _, cond := range nomc.Status.Conditions {
		if cond.Type == condType {
			if cond.Status == v1alpha1.ConditionInProgress {
				return false, nil
			}
			if cond.Status == v1alpha1.ConditionTrue {
				return true, nil
			}
		} else {
			if cond.Status == v1alpha1.ConditionTrue ||
				cond.Status == v1alpha1.ConditionInProgress {
				return false, fmt.Errorf("NodeObservabilityMachineConfig not in expected state. Condition: %s State: %s", cond.Type, cond.Status)
			}
		}
	}
	return false, fmt.Errorf("NodeObservabilityMachineConfig in unknown state")
}

func (r *NodeObservabilityRunReconciler) disableCrioKubeletProfile(ctx context.Context) error {
	nameSpace := types.NamespacedName{Name: MCOName}
	nomc, err := r.fetchNOMC(ctx, nameSpace)
	if err != nil {
		return err
	}

	if nomc.Spec.Debug.EnableCrioProfiling {
		nomc.Spec.Debug.EnableCrioProfiling = false
		return r.updateNOMC(ctx, nomc)
	}
	return nil
}
