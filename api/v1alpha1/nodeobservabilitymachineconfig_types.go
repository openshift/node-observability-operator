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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// NodeObservabilityMachineConfigSpec defines the desired state of NodeObservabilityMachineConfig
type NodeObservabilityMachineConfigSpec struct {
	// EnableCrioProfiling is for enabling profiling of CRI-O service
	EnableCrioProfiling bool `json:"enableCrioProfiling,omitempty"`

	// EnableKubeletProfiling is for enabling profiling of CRI-O service
	EnableKubeletProfiling bool `json:"enableKubeletProfiling,omitempty"`
}

// NodeObservabilityMachineConfigStatus defines the observed state of NodeObservabilityMachineConfig
type NodeObservabilityMachineConfigStatus struct {
	// lastUpdated is the timestamp of previous status update
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// UpdateStatus contains of the status of the MCP update
	UpdateStatus ConfigUpdateStatus `json:"updateStatus,omitempty"`
}

// ConfigUpdateStatus is for storing the status of the MCP update
type ConfigUpdateStatus struct {
	// InProgress is for tracking the update of machines in profiling MCP
	InProgress corev1.ConditionStatus `json:"InProgress,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// NodeObservabilityMachineConfig is the Schema for the nodeobservabilitymachineconfigs API
type NodeObservabilityMachineConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeObservabilityMachineConfigSpec   `json:"spec,omitempty"`
	Status NodeObservabilityMachineConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeObservabilityMachineConfigList contains a list of NodeObservabilityMachineConfig
type NodeObservabilityMachineConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeObservabilityMachineConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeObservabilityMachineConfig{}, &NodeObservabilityMachineConfigList{})
}
