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
	"encoding/json"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensureReqNodeLabelExists is for checking the if the required labels exist on the nodes
func (r *MachineConfigReconciler) ensureReqNodeLabelExists(ctx context.Context) (int, error) {

	updNodeCount := 0
	nodeList := &corev1.NodeList{}
	if err := r.listNodes(ctx, nodeList, r.CtrlConfig.Spec.NodeSelector); err != nil {
		return updNodeCount, err
	}

	for i, node := range nodeList.Items {
		if _, exist := node.Labels[NodeObservabilityNodeRoleLabelName]; exist {
			continue
		}

		patch, _ := newPatch(add,
			ResourceLabelsPath,
			map[string]interface{}{
				NodeObservabilityNodeRoleLabelName: Empty,
			})
		if err := r.ClientPatch(ctx, &nodeList.Items[i], client.RawPatch(types.JSONPatchType, patch)); err != nil {
			return updNodeCount, err
		}
		r.Log.V(1).Info("Successfully added label", "Node", node.Name, "Label", NodeObservabilityNodeRoleLabelName)

		updNodeCount++
	}

	if updNodeCount > 0 {
		r.Log.V(1).Info("Successfully added nodeobservability role to nodes with worker role", "NodeCount", updNodeCount)
	}
	return updNodeCount, nil
}

// ensureReqNodeLabelNotExists removes the nodeobservability label from the nodes.
// Returns the number of updated nodes.
func (r *MachineConfigReconciler) ensureReqNodeLabelNotExists(ctx context.Context) (int, error) {

	updNodeCount := 0
	nodeList := &corev1.NodeList{}
	if err := r.listNoObsLabeledNodes(ctx, nodeList); err != nil {
		return updNodeCount, err
	}

	for i, node := range nodeList.Items {
		if _, exist := node.Labels[NodeObservabilityNodeRoleLabelName]; !exist {
			continue
		}

		patch, _ := newPatch(remove,
			ResourceLabelsPath,
			map[string]interface{}{
				NodeObservabilityNodeRoleLabelName: Empty,
			})
		if err := r.ClientPatch(ctx, &nodeList.Items[i], client.RawPatch(types.JSONPatchType, patch)); err != nil {
			return updNodeCount, err
		}
		r.Log.V(1).Info("Successfully removed label", "Node", node.Name, "Label", NodeObservabilityNodeRoleLabelName)

		updNodeCount++
	}

	if updNodeCount > 0 {
		r.Log.V(1).Info("Successfully removed nodeobservability role from nodes with worker role", "NodeCount", updNodeCount)
	}
	return updNodeCount, nil
}

// listNoObsLabeledNodes returns the list of nodes having NodeObservability role label
func (r *MachineConfigReconciler) listNoObsLabeledNodes(ctx context.Context, nodeList *corev1.NodeList) error {
	nodeLabel := map[string]string{
		NodeObservabilityNodeRoleLabelName: Empty,
	}

	if err := r.listNodes(ctx, nodeList, nodeLabel); err != nil {
		return fmt.Errorf("failed to get the list of nodes labeled nodeobservability: %w", err)
	}

	return nil
}

// listNodes returns the list of nodes matching the labels
func (r *MachineConfigReconciler) listNodes(ctx context.Context, nodeList *corev1.NodeList, matchLabels map[string]string) error {
	listOpts := []client.ListOption{
		client.MatchingLabels(matchLabels),
	}

	if err := r.ClientList(ctx, nodeList, listOpts...); err != nil {
		return err
	}

	return nil
}

// escape replaces characters which would cause parsing issues with their escaped equivalent
func escape(key string) string {
	// The `/` in the metadata key needs to be escaped in order to not be considered a "directory" in the path
	return strings.Replace(key, "/", "~1", -1)
}

// newPatch returns the patch data in the format required by the client
func newPatch(op patchOp, pathPrefix string, patch map[string]interface{}) ([]byte, error) {
	if patch == nil {
		return nil, fmt.Errorf("patch data cannot be empty")
	}

	switch op {
	case add:
		return newAddPatch(pathPrefix, patch), nil
	case remove:
		return newRemovePatch(pathPrefix, patch), nil
	}
	return nil, fmt.Errorf("patch operation type[%v] not supported", op)
}

// newAddPatch returns the patch data to add in the format required by the client
func newAddPatch(pathPrefix string, patch map[string]interface{}) []byte {
	values := make([]ResourcePatchValue, 0, len(patch))
	for k, v := range patch {
		ppath := path.Join(pathPrefix, escape(k))
		values = append(values, getPatchValue("add", ppath, v))
	}

	data, _ := json.Marshal(values)
	return data
}

// newRemovePatch returns the patch data to remove in the format required by the client
func newRemovePatch(pathPrefix string, patch map[string]interface{}) []byte {
	values := make([]ResourcePatchValue, 0, len(patch))
	for k, v := range patch {
		ppath := path.Join(pathPrefix, escape(k))
		values = append(values, getPatchValue("remove", ppath, v))
	}

	data, _ := json.Marshal(values)
	return data
}

// getPatchValue returns the patch data in the required format
func getPatchValue(op, path string, value interface{}) ResourcePatchValue {
	return ResourcePatchValue{
		Op:    op,
		Path:  path,
		Value: value,
	}
}
