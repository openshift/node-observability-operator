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

	"github.com/openshift/node-observability-operator/api/v1alpha2"
)

const (
	patchAdd    = "add"
	patchRemove = "remove"
)

// ensureReqNodeLabelExists updates the given nodes with the required labels.
func (r *MachineConfigReconciler) ensureReqNodeLabelExists(ctx context.Context, nodeList *corev1.NodeList) error {

	nodeCount := 0
	requiredNodeCount := len(nodeList.Items)
	for i, node := range nodeList.Items {
		if _, exist := node.Labels[NodeObservabilityNodeRoleLabelName]; exist {
			nodeCount++
			continue
		}

		err := r.patchNodeLabels(ctx, &nodeList.Items[i], patchAdd, ResourceLabelsPath, map[string]interface{}{NodeObservabilityNodeRoleLabelName: empty})
		if err != nil {
			return fmt.Errorf("failed to patch node due to %w", err)
		}
		r.Log.V(1).Info("successfully performed add operation for labels", "node.name", node.Name, "node.label", NodeObservabilityNodeRoleLabelName)
		nodeCount++
	}

	if nodeCount == requiredNodeCount {
		r.Log.V(1).Info("nodeobservability role is present on all the nodes with worker role", "updatednodecount", nodeCount)
		return nil
	}

	return fmt.Errorf("failed to ensure required label, expected updates on %d nodes but got %d", requiredNodeCount, nodeCount)
}

// ensureReqNodeLabelNotExists updates the given nodes to ensure all the
// required labels are removed.
func (r *MachineConfigReconciler) ensureReqNodeLabelNotExists(ctx context.Context, nodeList *corev1.NodeList) error {

	nodeCount := 0
	requiredNodeCount := len(nodeList.Items)
	for i, node := range nodeList.Items {
		if _, exist := node.Labels[NodeObservabilityNodeRoleLabelName]; !exist {
			continue
		}

		err := r.patchNodeLabels(ctx, &nodeList.Items[i], patchRemove, ResourceLabelsPath, map[string]interface{}{NodeObservabilityNodeRoleLabelName: empty})
		if err != nil {
			return fmt.Errorf("failed to patch node due to %w", err)
		}
		r.Log.V(1).Info("successfully performed remove operation for labels", "Node", node.Name, "Label", NodeObservabilityNodeRoleLabelName)
		nodeCount++
	}

	if nodeCount == requiredNodeCount {
		r.Log.V(1).Info("nodeobservability role is removed on all the nodes with worker role", "UpdatedNodeCount", nodeCount)
		return nil
	}

	r.Log.V(1).Info("nodeobservability role is not removed on all the nodes with worker role", "Nodes", requiredNodeCount, "UpdatedNodeCount", nodeCount)
	return fmt.Errorf("failed to ensure removal of labels, expected updates on %d nodes but got %d", requiredNodeCount, nodeCount)
}

// listNodes returns the list of nodes matching the labels
func (r *MachineConfigReconciler) listNodes(ctx context.Context, matchLabels map[string]string) (*corev1.NodeList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(matchLabels),
	}

	nodeList := &corev1.NodeList{}
	if err := r.ClientList(ctx, nodeList, listOpts...); err != nil {
		return nil, err
	}

	return nodeList, nil
}

// ensureProfConfEnabled makes sure all the configuration needed to enable the
// CRI-O profiling is applied.
func (r *MachineConfigReconciler) ensureProfConfEnabled(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) error {

	nodeList, err := r.listNodes(ctx, nomc.Spec.NodeSelector)
	if err != nil {
		return err
	}

	if len(nodeList.Items) == 0 {
		r.Log.V(1).Info("no nodes matching the given selector were found", "nodeselector", nomc.Spec.NodeSelector)
		return nil
	}

	err = r.ensureReqNodeLabelExists(ctx, nodeList)
	if err != nil {
		return fmt.Errorf("failed to ensure nodes are labelled: %w", err)
	}

	if err := r.enableCrioProf(ctx, nomc); err != nil {
		return fmt.Errorf("failed to enable CRIO mc: %w", err)
	}

	if err := r.createProfMCP(ctx, nomc); err != nil {
		return fmt.Errorf("failed to create profiling mcp: %w", err)
	}

	return nil
}

// ensureProfConfDisabled disables the profiling on the requested nodes by removing the nodeobservability label.
func (r *MachineConfigReconciler) ensureProfConfDisabled(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) error {

	nodeList, err := r.listNodes(ctx, map[string]string{NodeObservabilityNodeRoleLabelName: empty})
	if err != nil {
		return fmt.Errorf("failed to get the list of nodes labelled nodeobservability: %w", err)
	}

	err = r.ensureReqNodeLabelNotExists(ctx, nodeList)
	if err != nil {
		return fmt.Errorf("failed to ensure nodes are not labelled: %w", err)
	}

	return nil
}

// escape replaces characters which would cause parsing issues with their escaped equivalent
func escape(key string) string {
	// The `/` in the metadata key needs to be escaped in order to not be considered a "directory" in the path
	return strings.Replace(key, "/", "~1", -1)
}

// patchNodeLabels creates and patches the given node with the provided labels
func (r *MachineConfigReconciler) patchNodeLabels(ctx context.Context, node *corev1.Node, op string, pathPrefix string, labels map[string]interface{}) error {
	if labels == nil {
		return fmt.Errorf("patch data cannot be empty")
	}

	values := make([]ResourcePatchValue, 0, len(labels))
	for k, v := range labels {
		resourcePath := path.Join(pathPrefix, escape(k))
		values = append(values, ResourcePatchValue{
			Op:    op,
			Path:  resourcePath,
			Value: v,
		})
	}

	data, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("unable to marshal patch values: %w", err)
	}

	return r.ClientPatch(ctx, node, client.RawPatch(types.JSONPatchType, data))
}
