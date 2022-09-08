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

package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

func (r *NodeObservabilityReconciler) ensureNOMC(ctx context.Context, instance *v1alpha1.NodeObservability, ns string) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	nameSpace := types.NamespacedName{Name: instance.Name, Namespace: ns}

	desired := r.desiredNOMC(instance, nameSpace)
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set the controller reference for nomc %q: %w", nameSpace.Name, err)
	}

	currentNOMC, err := r.currentNOMC(ctx, nameSpace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get nomc %s/%s due to: %w", nameSpace.Namespace, nameSpace.Name, err)
	} else if err != nil && errors.IsNotFound(err) {

		// create NOMC since it doesn't exist
		if err := r.createNOMC(ctx, desired); err != nil {
			return nil, err
		}
		return r.currentNOMC(ctx, nameSpace)
	}

	return r.updateNOMC(ctx, currentNOMC, desired)
}

// currentNOMC checks if the NodeObservabilityMachineConfig exists
func (r *NodeObservabilityReconciler) currentNOMC(ctx context.Context, nameSpace types.NamespacedName) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	mc := &v1alpha1.NodeObservabilityMachineConfig{}
	if err := r.Get(ctx, nameSpace, mc); err != nil {
		return nil, err
	}
	return mc, nil
}

// desiredNOMC returns a NodeObservabilityMachineConfig object
func (r *NodeObservabilityReconciler) desiredNOMC(instance *v1alpha1.NodeObservability, nameSpace types.NamespacedName) *v1alpha1.NodeObservabilityMachineConfig {
	return &v1alpha1.NodeObservabilityMachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: nameSpace.Namespace,
		},
		Spec: r.desiredNOMCSpec(instance),
	}
}

// createNOMC creates the NodeObservabilityMachineConfig
func (r *NodeObservabilityReconciler) createNOMC(ctx context.Context, instance *v1alpha1.NodeObservabilityMachineConfig) error {
	if err := r.Create(ctx, instance); err != nil {
		return fmt.Errorf("failed to create nomc %s/%s: %w", instance.Namespace, instance.Name, err)
	}
	r.Log.V(1).Info("created nomc", "nomc.namespace", instance.Namespace, "nomc.name", instance.Name)
	return nil
}

// desiredNOMCSpec returns a NodeObservabilityMachineConfigSpec object
func (r *NodeObservabilityReconciler) desiredNOMCSpec(instance *v1alpha1.NodeObservability) v1alpha1.NodeObservabilityMachineConfigSpec {
	s := v1alpha1.NodeObservabilityMachineConfigSpec{}
	if instance.Spec.Type == v1alpha1.CrioKubeletNodeObservabilityType {
		s.Debug.EnableCrioProfiling = true
	}
	if len(instance.Spec.NodeSelector) != 0 {
		s.NodeSelector = instance.Spec.NodeSelector
	}
	// TODO: ebpf, custom will go here
	return s
}

func (r *NodeObservabilityReconciler) updateNOMC(ctx context.Context, current, desired *v1alpha1.NodeObservabilityMachineConfig) (*v1alpha1.NodeObservabilityMachineConfig, error) {
	updatedNOMC := current.DeepCopy()
	updated := false

	if !cmp.Equal(current.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		updatedNOMC.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
		updated = true
	}

	if current.Spec.Debug.EnableCrioProfiling != desired.Spec.Debug.EnableCrioProfiling {
		updatedNOMC.Spec.Debug.EnableCrioProfiling = desired.Spec.Debug.EnableCrioProfiling
		updated = true
	}

	if updated {
		return updatedNOMC, r.Update(ctx, updatedNOMC)
	}

	return updatedNOMC, nil
}

func (r *NodeObservabilityReconciler) deleteNOMC(ctx context.Context, nodeObs *v1alpha1.NodeObservability) error {
	mc := &v1alpha1.NodeObservabilityMachineConfig{}
	mc.Name = nodeObs.Name
	if err := r.Delete(ctx, mc); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete nomc %s/%s: %w", mc.Namespace, mc.Name, err)
	}
	r.Log.V(1).Info("deleted nomc", "nomc.name", mc.Name)
	return nil
}
