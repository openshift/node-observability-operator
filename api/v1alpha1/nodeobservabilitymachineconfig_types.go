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

// NodeObservabilityMachineConfigConditionType is the different type of conditions
type NodeObservabilityMachineConfigConditionType string

const (
	// DebugEnabled is the condition used to inform state of enabling the
	// debugging configuration for the requested services
	DebugEnabled NodeObservabilityMachineConfigConditionType = "DebugEnabled"

	// DebugDisabled is the condition used to inform state of disabling the
	// debugging configuration for the requested services
	DebugDisabled NodeObservabilityMachineConfigConditionType = "DebugDisabled"

	// Failed is the condition used to inform failed state while enabling or
	// disabling the debugging configuration for the requested services
	Failed NodeObservabilityMachineConfigConditionType = "Failed"
)

// ConditionStatus is the different condition states
type ConditionStatus string

const (
	// ConditionTrue means the resource is in the defined condition
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse means the resource is not in the defined condition
	ConditionFalse ConditionStatus = "False"

	// ConditionInProgress means the resource is in progress of being
	// in the defined condition
	ConditionInProgress ConditionStatus = "InProgress"

	// ConditionUnknown means the resource is in the undetermined condition
	ConditionUnknown ConditionStatus = "Unknown"
)

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
	Conditions []NodeObservabilityMachineConfigCondition `json:"conditions"`
}

// NodeObservabilityMachineConfigCondition is for storing the status of the MCP update
type NodeObservabilityMachineConfigCondition struct {
	// type specifies the state of the operator's reconciliation functionality.
	Type NodeObservabilityMachineConfigConditionType `json:"type"`

	// status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`

	// lastUpdateTime is the time last update was applied.
	// +nullable
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`

	// message provides additional information about the current condition.
	Message string `json:"message,omitempty"`
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
