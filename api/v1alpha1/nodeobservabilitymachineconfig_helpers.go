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

// UpdateLastReconcileTime is for updating LastReconcile in NodeObservabilityMachineConfigStatus
func UpdateLastReconcileTime(status *NodeObservabilityMachineConfigStatus) {
	status.LastReconcile = metav1.Now()
}

// NewNodeObservabilityMachineConfigCondition creates a new NodeObservabilityMachineConfig condition.
func NewNodeObservabilityMachineConfigCondition(
	condType NodeObservabilityMachineConfigConditionType,
	status ConditionStatus,
	message string) *NodeObservabilityMachineConfigCondition {

	return &NodeObservabilityMachineConfigCondition{
		Type:           condType,
		Status:         status,
		LastUpdateTime: metav1.Now(),
		Message:        message,
	}
}

// GetNodeObservabilityMachineConfigCondition returns the condition with the provided type.
func GetNodeObservabilityMachineConfigCondition(
	status NodeObservabilityMachineConfigStatus,
	condType NodeObservabilityMachineConfigConditionType) *NodeObservabilityMachineConfigCondition {

	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetNodeObservabilityMachineConfigCondition updates the NodeObservabilityMachineConfig to include the provided condition. If the condition
// already exists and has the same status and no changes are made.
func SetNodeObservabilityMachineConfigCondition(
	status *NodeObservabilityMachineConfigStatus,
	condition NodeObservabilityMachineConfigCondition) {

	currentCond := GetNodeObservabilityMachineConfigCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Message == condition.Message {
		return
	}
	newConditions := filterOutNodeObservabilityMachineConfigCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)

	for i := range status.Conditions {
		if status.Conditions[i].Type != condition.Type &&
			status.Conditions[i].Status != ConditionFalse {
			(&status.Conditions[i]).Status = ConditionFalse
		}
	}
}

// RemoveNodeObservabilityMachineConfigCondition removes the NodeObservabilityMachineConfig condition with the provided type.
func RemoveNodeObservabilityMachineConfigCondition(
	status *NodeObservabilityMachineConfigStatus,
	condType NodeObservabilityMachineConfigConditionType) {
	status.Conditions = filterOutNodeObservabilityMachineConfigCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of NodeObservabilityMachineConfig conditions without conditions with the provided type.
func filterOutNodeObservabilityMachineConfigCondition(
	conditions []NodeObservabilityMachineConfigCondition,
	condType NodeObservabilityMachineConfigConditionType) []NodeObservabilityMachineConfigCondition {
	var newConditions []NodeObservabilityMachineConfigCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// IsNodeObservabilityMachineConfigConditionTrue returns true when the conditionType is present and set to `ConditionTrue`
func IsNodeObservabilityMachineConfigConditionTrue(conditions []NodeObservabilityMachineConfigCondition, conditionType NodeObservabilityMachineConfigConditionType) bool {
	return IsNodeObservabilityMachineConfigConditionPresentAndEqual(conditions, conditionType, ConditionTrue)
}

// IsNodeObservabilityMachineConfigConditionInProgress returns true when the conditionType is present and set to `ConditionInProgress`
func IsNodeObservabilityMachineConfigConditionInProgress(conditions []NodeObservabilityMachineConfigCondition, conditionType NodeObservabilityMachineConfigConditionType) bool {
	return IsNodeObservabilityMachineConfigConditionPresentAndEqual(conditions, conditionType, ConditionInProgress)
}

// IsNodeObservabilityMachineConfigConditionSetInProgress returns first encountered conditionType which is set to `ConditionInProgress`
func IsNodeObservabilityMachineConfigConditionSetInProgress(conditions []NodeObservabilityMachineConfigCondition) NodeObservabilityMachineConfigConditionType {
	for _, condition := range conditions {
		if condition.Status == ConditionInProgress {
			return condition.Type
		}
	}
	return ""
}

// IsNodeObservabilityMachineConfigConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsNodeObservabilityMachineConfigConditionPresentAndEqual(conditions []NodeObservabilityMachineConfigCondition, conditionType NodeObservabilityMachineConfigConditionType, status ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
