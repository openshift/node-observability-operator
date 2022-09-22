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

// +kubebuilder:validation:Enum=crio-kubelet;
type NodeObservabilityType string

const (
	CrioKubeletNodeObservabilityType NodeObservabilityType = "crio-kubelet"
)

// NodeObservabilitySpec defines the desired state of NodeObservability
type NodeObservabilitySpec struct {
	// Labels is map of key:value pairs that are used to match against node labels
	Labels map[string]string `json:"labels,omitempty"`
	// +kubebuilder:validation:Required
	// Type defines the type of profiling queries, which will be enabled
	// The following types are supported:
	//   * crio-kubelet - 30s of /pprof data, requesting this type might cause node restart
	Type NodeObservabilityType `json:"type"`
}

// NodeObservabilityStatus defines the observed state of NodeObservability
type NodeObservabilityStatus struct {
	// Count is the number of pods (one for each node) the daemon is deployed to
	Count      int32        `json:"count"`
	LastUpdate *metav1.Time `json:"lastUpdated,omitempty"`
	// Conditions contain details for aspects of the current state of this API Resource.
	ConditionalStatus `json:"conditions,omitempty"`
}

//+kubebuilder:resource:scope=Cluster,shortName=nob
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeObservability prepares a subset of worker nodes (identified
// by a label) for running node observability queries, such as profiling
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

// IsDebuggingFailed returns true if Debugging has failed
func (s *NodeObservabilityStatus) IsReady() bool {
	if cond := s.GetCondition(DebugReady); cond != nil && cond.Status == metav1.ConditionTrue {
		return true
	}
	return false
}

func init() {
	SchemeBuilder.Register(&NodeObservability{}, &NodeObservabilityList{})
}
