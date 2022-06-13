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
	"k8s.io/client-go/util/retry"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

func (r *NodeObservabilityReconciler) ensureNOMC(ctx context.Context, instance *v1alpha1.NodeObservability) (bool, *v1alpha1.NodeObservabilityMachineConfig, error) {
	desired := r.desiredNOMC(instance)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, corErr := ctrlutil.CreateOrUpdate(ctx, r.Client, desired, func() error {
			desired.Spec = r.desiredNOMCSpec(instance)
			return ctrlutil.SetControllerReference(instance, desired, r.Scheme)
		})
		return corErr
	})
	if err != nil {
		return false, nil, err
	}
	return r.currentNOMC(ctx, types.NamespacedName{Name: instance.Name})
}

// currentNOMC checks if the NodeObservabilityMachineConfig exists
func (r *NodeObservabilityReconciler) currentNOMC(ctx context.Context, nameSpace types.NamespacedName) (bool, *v1alpha1.NodeObservabilityMachineConfig, error) {
	mc := &v1alpha1.NodeObservabilityMachineConfig{}
	if err := r.Get(ctx, nameSpace, mc); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, mc, nil
}

// desiredNOMC returns a NodeObservabilityMachineConfig object
func (r *NodeObservabilityReconciler) desiredNOMC(instance *v1alpha1.NodeObservability) *v1alpha1.NodeObservabilityMachineConfig {
	return &v1alpha1.NodeObservabilityMachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
		Spec: r.desiredNOMCSpec(instance),
	}
}

// desiredNOMCSpec returns a NodeObservabilityMachineConfigSpec object
func (r *NodeObservabilityReconciler) desiredNOMCSpec(instance *v1alpha1.NodeObservability) v1alpha1.NodeObservabilityMachineConfigSpec {
	s := v1alpha1.NodeObservabilityMachineConfigSpec{}
	if instance.Spec.Type == v1alpha1.CrioKubeletNodeObservabilityType {
		s.Debug.EnableCrioProfiling = true
	}
	// TODO: ebpf, custom will go here
	return s
}

func (r *NodeObservabilityReconciler) deleteNOMC(ctx context.Context, nodeObs *v1alpha1.NodeObservability) error {
	mc := &v1alpha1.NodeObservabilityMachineConfig{}
	mc.Name = nodeObs.Name
	if err := r.Delete(ctx, mc); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete NodeObservabilityMachineConfig %s: %w", mc.Name, err)
	}
	r.Log.Info("Deleted NodeObservabilityMachinConfig", "name", mc.Name)
	return nil
}
