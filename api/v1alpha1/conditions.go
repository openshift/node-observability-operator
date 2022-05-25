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

const (
	// DebugEnabled is the condition type used to inform state of enabling the
	// debugging configuration for the requested services
	// DebugEnabled has the following options:
	//   Status:
	//   - True
	//   - False
	//   Reason:
	//   - Enabled
	//   - Disabled
	DebugEnabled string = "DebugEnabled"

	// DebugReady is the condition type used to inform state of readiness of the
	// debugging configuration for the requested services
	//   Status:
	//   - True
	//   - False
	//   Reason:
	//   - In progress
	//   - Failed
	//   - Ready: config successfully applied and ready
	DebugReady string = "Ready"
)

const (
	ReasonEnabled string = "Enabled"

	ReasonDisabled string = "Disabled"

	ReasonReady string = "Ready"

	ReasonFailed string = "Failed"

	ReasonInProgress string = "InProgress"
)

type ConditionalStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

func (c *ConditionalStatus) GetCondition(t string) *metav1.Condition {
	for i, cond := range c.Conditions {
		if cond.Type == t {
			return &c.Conditions[i]
		}
	}
	return nil
}

func (c *ConditionalStatus) SetCondition(t string, cs metav1.ConditionStatus, reason, msg string) bool {
	condition := c.GetCondition(t)
	if condition == nil {
		c.Conditions = append(c.Conditions, metav1.Condition{
			Type:               t,
			Status:             cs,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            msg,
		})
		return true
	}
	if condition.Status != cs || condition.Reason != reason {
		condition.Status = cs
		condition.LastTransitionTime = metav1.Now()
		condition.Reason = reason
		condition.Message = msg
		return true
	}
	return false
}
