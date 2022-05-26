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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// NodeObservabilityMachineConfigSpec defines the desired state of NodeObservabilityMachineConfig
type NodeObservabilityMachineConfigSpec struct {
	Debug NodeObservabilityDebug `json:"debug,omitempty"`
}

// NodeObservabilityDebug is for holding the configurations defined for
// enabling debugging of services
type NodeObservabilityDebug struct {
	// EnableCrioProfiling is for enabling profiling of CRI-O service
	EnableCrioProfiling bool `json:"enableCrioProfiling,omitempty"`
}

// NodeObservabilityMachineConfigStatus defines the observed state of NodeObservabilityMachineConfig
type NodeObservabilityMachineConfigStatus struct {
	// conditions represents the latest available observations of current operator state.
	// +optional
	ConditionalStatus `json:"conditions"`

	// lastReconcile is the time of last reconciliation
	// +nullable
	LastReconcile metav1.Time `json:"lastReconcile"`
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

// IsMachineConfigInProgress returns true if MachineConfig configuration is being applied
func (s *NodeObservabilityMachineConfigStatus) IsMachineConfigInProgress() bool {
	if cond := s.GetCondition(DebugReady); cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == ReasonInProgress {
		return true
	}
	return false
}

// IsDebuggingFailed returns true if Debugging has failed
func (s *NodeObservabilityMachineConfigStatus) IsDebuggingFailed() bool {
	if cond := s.GetCondition(DebugReady); cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == ReasonFailed {
		return true
	}
	return false
}

// IsDebuggingEnabled returns true if Debugging is enabled
func (s *NodeObservabilityMachineConfigStatus) IsDebuggingEnabled() bool {
	if cond := s.GetCondition(DebugEnabled); cond != nil && cond.Status == metav1.ConditionTrue {
		return true
	}
	return false
}

// UpdateLastReconcileTime is for updating LastReconcile in NodeObservabilityMachineConfigStatus
func (status *NodeObservabilityMachineConfigStatus) UpdateLastReconcileTime() {
	status.LastReconcile = metav1.Now()
}

func init() {
	SchemeBuilder.Register(&NodeObservabilityMachineConfig{}, &NodeObservabilityMachineConfigList{})
}
