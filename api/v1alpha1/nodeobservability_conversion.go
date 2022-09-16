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

var nobWebhookLog = logf.Log.WithName("nob-conversion-webhook")

// ConvertTo converts this NodeObservability to the Hub version (v1alpha2).
func (src *NodeObservability) ConvertTo(dstRaw conversion.Hub) error {
	nobWebhookLog.Info("Converting to v1alpha2", "name", src.Name)

	dst := dstRaw.(*v1alpha2.NodeObservability)

	dst.Spec.NodeSelector = src.Spec.Labels

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Type = v1alpha2.NodeObservabilityType(src.Spec.Type)

	// Status
	dst.Status.Count = src.Status.Count
	dst.Status.LastUpdate = src.Status.LastUpdate
	dst.Status.ConditionalStatus = v1alpha2.ConditionalStatus(src.Status.ConditionalStatus)

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *NodeObservability) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.NodeObservability)

	nobWebhookLog.Info("Converting to v1alpha1", "name", src.Name)

	dst.Spec.Labels = src.Spec.NodeSelector

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Type = NodeObservabilityType(src.Spec.Type)

	// Status
	dst.Status.Count = src.Status.Count
	dst.Status.LastUpdate = src.Status.LastUpdate
	dst.Status.ConditionalStatus = ConditionalStatus(src.Status.ConditionalStatus)

	return nil
}
