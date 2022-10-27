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

package ca_configmap

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	opctrl "github.com/openshift/node-observability-operator/pkg/operator/controller"
)

// ensureKubeletCAConfigMap ensures that the source configmap has been copied to the operand namespace.
// Returns the target configmap, a boolean if the target configmap exists, and an error when relevant.
func (r *reconciler) ensureKubeletCAConfigMap(ctx context.Context) error {
	// get the source configmap
	srcName := types.NamespacedName{Namespace: r.config.SourceNamespace, Name: r.config.CAConfigMapName}
	sourceExists, source, err := r.currentKubeletCAConfigMap(ctx, srcName)
	if err != nil {
		return err
	} else if !sourceExists {
		return nil
	}

	targetName := opctrl.NamespacedKubeletCAConfigMapName(r.config.TargetNamespace)
	targetExists, target, err := r.currentKubeletCAConfigMap(ctx, targetName)
	if err != nil {
		return err
	}

	// desired is created from the source
	desired := desiredKubeletCAConfigMap(source, targetName)

	if !targetExists {
		// target configmap doesn't exist, create it
		if err := r.createKubeletCAConfigMap(ctx, desired); err != nil {
			return err
		}
		return nil
	}

	// target configmap exists, try to update it with the source data
	if updated, err := r.updateKubeletCAConfigMap(ctx, target, desired); err != nil {
		return err
	} else if updated {
		return nil
	}

	return nil
}

// currentKubeletCAConfigMap returns the definition of the configmap object with the given name.
func (r *reconciler) currentKubeletCAConfigMap(ctx context.Context, name types.NamespacedName) (bool, *corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	if err := r.client.Get(ctx, name, cm); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, cm, nil
}

// createKubeletCAConfigMap creates the given configmap.
func (r *reconciler) createKubeletCAConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	if err := r.client.Create(ctx, cm); err != nil {
		return err
	}
	r.log.Info("kubelet CA configmap created", "cm.namespace", cm.Namespace, "cm.name", cm.Name)
	return nil
}

// desiredKubeletCAConfigMap returns the desired target configmap.
func desiredKubeletCAConfigMap(source *corev1.ConfigMap, targetName types.NamespacedName) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName.Name,
			Namespace: targetName.Namespace,
		},
		Data: source.Data,
	}
}

// updateKubeletCAConfigMap updates the target configmap with the desired content if update is needed.
// Returns a boolean indicating whether the configmap was updated, and an error value if any occurred.
func (r *reconciler) updateKubeletCAConfigMap(ctx context.Context, current, desired *corev1.ConfigMap) (bool, error) {
	if configMapsEqual(current, desired) {
		return false, nil
	}
	updated := current.DeepCopy()
	updated.Data = desired.Data
	if err := r.client.Update(ctx, updated); err != nil {
		return false, err
	}
	r.log.Info("kubelet CA configmap updated", "cm.namespace", updated.Namespace, "cm.name", updated.Name)
	return true, nil
}

// configMapsEqual compares two configMaps. Returns true if
// the configMaps should be considered equal for the purpose of determining
// whether an update is necessary, false otherwise.
func configMapsEqual(a, b *corev1.ConfigMap) bool {
	return reflect.DeepEqual(a.Data, b.Data)
}
