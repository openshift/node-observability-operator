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
package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openshift/node-observability-operator/api/v1alpha2"
)

var webhookNobmcLog = logf.Log.WithName("nobmc-conversion-webhook")

// ConvertTo converts this NodeObservabilityMachineConfig to the Hub version (v1alpha2).
func (src *NodeObservabilityMachineConfig) ConvertTo(dstRaw conversion.Hub) error {
	webhookNobmcLog.V(1).Info("converting to v1alpha2", "name", src.Name)

	dst := dstRaw.(*v1alpha2.NodeObservabilityMachineConfig)

	// all worker nodes
	dst.Spec.NodeSelector = map[string]string{
		"node-role.kubernetes.io/worker": "",
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Debug = v1alpha2.NodeObservabilityDebug(src.Spec.Debug)

	// Status
	dst.Status.LastReconcile = src.Status.LastReconcile
	dst.Status.ConditionalStatus = v1alpha2.ConditionalStatus(src.Status.ConditionalStatus)

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *NodeObservabilityMachineConfig) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.NodeObservabilityMachineConfig)

	webhookNobmcLog.V(1).Info("converting to v1alpha1", "name", src.Name)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Debug = NodeObservabilityDebug(src.Spec.Debug)
	dst.Spec.Debug = NodeObservabilityDebug(src.Spec.Debug)
	// we are losing the nodeselector as it's only available in v1alpha2

	// Status
	dst.Status.LastReconcile = src.Status.LastReconcile
	dst.Status.ConditionalStatus = ConditionalStatus(src.Status.ConditionalStatus)

	return nil
}
