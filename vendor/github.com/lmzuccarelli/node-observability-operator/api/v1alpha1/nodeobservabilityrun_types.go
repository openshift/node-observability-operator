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

// NodeObservabilityRunSpec defines the desired state of NodeObservabilityRun
type NodeObservabilityRunSpec struct {

	// NodeObservabilityRef is the reference to the parent NodeObservability resource
	NodeObservabilityRef *NodeObservabilityRef `json:"nodeObservabilityRef"`
}

// NodeObservabilityRef is the reference to the parent NodeObservability resource
type NodeObservabilityRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
}

// NodeObservabilityRunStatus defines the observed state of NodeObservabilityRun
type NodeObservabilityRunStatus struct {
	// StartTimestamp represents the server time when the NodeObservabilityRun started.
	// When not set, the NodeObservabilityRun hasn't started.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// FinishedTimestamp represents the server time when the NodeObservabilityRun finished.
	// When not set, the NodeObservabilityRun isn't known to have finished.
	// It is represented in RFC3339 form and is in UTC.
	FinishedTimestamp *metav1.Time `json:"finishedTimestamp,omitempty"`

	// Conditions contain details for aspects of the current state of this API Resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Output is the output location of this NodeObservabilityRun
	// When not set, no output location is known
	Output *string `json:"output,omitempty"`
}

// +kubebuilder:printcolumn:JSONPath=".spec.nodeObservabilityRef.name", name="NodeObservabilityRef", type="string"
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeObservabilityRun is the Schema for the nodeobservabilityruns API
type NodeObservabilityRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeObservabilityRunSpec   `json:"spec,omitempty"`
	Status NodeObservabilityRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeObservabilityRunList contains a list of NodeObservabilityRun
type NodeObservabilityRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeObservabilityRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeObservabilityRun{}, &NodeObservabilityRunList{})
}
