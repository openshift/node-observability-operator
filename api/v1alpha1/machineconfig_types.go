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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MachineconfigSpec defines the desired state of Machineconfig
type MachineconfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// MachineconfigStatus defines the observed state of Machineconfig
type MachineconfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// UpdateStatus contains of the status of the MCP update
	UpdateStatus ConfigUpdateStatus `json:"updateStatus,omitempty"`
}

// ConfigUpdateStatus is for storing the status of the MCP update
type ConfigUpdateStatus struct {
	InProgress corev1.ConditionStatus `json:"InProgress,omit"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Machineconfig is the Schema for the machineconfigs API
type Machineconfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineconfigSpec   `json:"spec,omitempty"`
	Status MachineconfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MachineconfigList contains a list of Machineconfig
type MachineconfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machineconfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Machineconfig{}, &MachineconfigList{})
}
