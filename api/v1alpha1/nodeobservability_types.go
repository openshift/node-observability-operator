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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeObservabilitySpec defines the desired state of NodeObservability
type NodeObservabilitySpec struct {
	// Labels is map of key:value pairs that are used to match against node labels
	Labels map[string]string `json:"labels,omitempty"`
	// Image is the container (pod) image to execute specific scripts on each node
	Image string `json:"image"`
}

// NodeObservabilityStatus defines the observed state of NodeObservability
type NodeObservabilityStatus struct {
	// Count is the number of pods (one for each node) the daemon is deployed to
	Count      int          `json:"count"`
	LastUpdate *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeObservability is the Schema for the nodeobservabilities API
type NodeObservability struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeObservabilitySpec   `json:"spec,omitempty"`
	Status NodeObservabilityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeObservabilityList contains a list of NodeObservability
type NodeObservabilityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeObservability `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeObservability{}, &NodeObservabilityList{})
}
