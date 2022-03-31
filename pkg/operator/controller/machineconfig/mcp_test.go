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

package machineconfigcontroller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func TestEnsureProfilingMCPExists(t *testing.T) {

	nodeObsMC := testNodeObsMC()
	mcp := getProfilingMCP()
	r := testReconciler()
	r.CtrlConfig = nodeObsMC

	tests := []struct {
		name    string
		reqObjs []runtime.Object
		preReq  func(*MachineConfigReconciler)
		wantErr bool
	}{
		{
			name:    "profiling MCP does not exist",
			reqObjs: []runtime.Object{nodeObsMC},
			wantErr: false,
		},
		{
			name:    "profiling MCP exists",
			reqObjs: []runtime.Object{nodeObsMC, mcp},
			wantErr: false,
		},
		{
			name:    "remove profiling MCP",
			reqObjs: []runtime.Object{nodeObsMC, mcp},
			preReq: func(r *MachineConfigReconciler) {
				r.CtrlConfig.Spec.EnableCrioProfiling = false
				r.CtrlConfig.Spec.EnableKubeletProfiling = false
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		if tt.preReq != nil {
			tt.preReq(r)
		}

		c := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tt.reqObjs...).Build()
		r.Client = c

		t.Run(tt.name, func(t *testing.T) {
			if _, err := r.ensureProfilingMCPExists(context.TODO()); (err != nil) != tt.wantErr {
				t.Errorf("ensureProfilingMCPExists() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckMCPUpdateStatus(t *testing.T) {

	nodeObsMC := testNodeObsMC()
	mcp := getProfilingMCP()
	r := testReconciler()
	r.CtrlConfig = nodeObsMC

	tests := []struct {
		name    string
		reqObjs []runtime.Object
		preReq  func(*MachineConfigReconciler, *mcv1.MachineConfigPool)
		wantErr bool
	}{
		{
			name:    "profiling MCP does not exist",
			reqObjs: []runtime.Object{},
			wantErr: false,
		},
		{
			name:    "no machine configs are updated",
			reqObjs: []runtime.Object{mcp},
			preReq: func(r *MachineConfigReconciler, mcp *mcv1.MachineConfigPool) {
				mcp.Status = mcv1.MachineConfigPoolStatus{
					MachineCount: 1,
				}
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{
					UpdateStatus: v1alpha1.ConfigUpdateStatus{
						InProgress: corev1.ConditionFalse,
					},
				}
			},
			wantErr: false,
		},
		{
			name:    "machine configs update in progress",
			reqObjs: []runtime.Object{mcp},
			preReq: func(r *MachineConfigReconciler, mcp *mcv1.MachineConfigPool) {
				mcp.Status = mcv1.MachineConfigPoolStatus{
					MachineCount: 1,
					Conditions: []mcv1.MachineConfigPoolCondition{
						{
							Status: corev1.ConditionTrue,
							Type:   mcv1.MachineConfigPoolUpdating,
						},
					},
				}
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{
					UpdateStatus: v1alpha1.ConfigUpdateStatus{
						InProgress: corev1.ConditionFalse,
					},
				}
			},
			wantErr: false,
		},
		{
			name:    "machine configs update not completed on all machines",
			reqObjs: []runtime.Object{mcp},
			preReq: func(r *MachineConfigReconciler, mcp *mcv1.MachineConfigPool) {
				mcp.Status = mcv1.MachineConfigPoolStatus{
					MachineCount: 1,
					Conditions: []mcv1.MachineConfigPoolCondition{
						{
							Status: corev1.ConditionTrue,
							Type:   mcv1.MachineConfigPoolUpdated,
						},
					},
				}
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{
					UpdateStatus: v1alpha1.ConfigUpdateStatus{
						InProgress: corev1.ConditionFalse,
					},
				}
			},
			wantErr: false,
		},
		{
			name:    "machine configs update completed",
			reqObjs: []runtime.Object{mcp},
			preReq: func(r *MachineConfigReconciler, mcp *mcv1.MachineConfigPool) {
				mcp.Status = mcv1.MachineConfigPoolStatus{
					MachineCount:        1,
					UpdatedMachineCount: 1,
					Conditions: []mcv1.MachineConfigPoolCondition{
						{
							Status: corev1.ConditionTrue,
							Type:   mcv1.MachineConfigPoolUpdated,
						},
					},
				}
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{
					UpdateStatus: v1alpha1.ConfigUpdateStatus{
						InProgress: corev1.ConditionFalse,
					},
				}
			},
			wantErr: false,
		},
		{
			name:    "machine configs update, machines in degraded state",
			reqObjs: []runtime.Object{mcp},
			preReq: func(r *MachineConfigReconciler, mcp *mcv1.MachineConfigPool) {
				mcp.Status = mcv1.MachineConfigPoolStatus{
					MachineCount:         1,
					DegradedMachineCount: 1,
					Conditions: []mcv1.MachineConfigPoolCondition{
						{
							Status: corev1.ConditionTrue,
							Type:   mcv1.MachineConfigPoolUpdated,
						},
					},
				}
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{
					UpdateStatus: v1alpha1.ConfigUpdateStatus{
						InProgress: corev1.ConditionFalse,
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		if tt.preReq != nil {
			tt.preReq(r, mcp)
		}

		c := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tt.reqObjs...).Build()
		r.Client = c

		t.Run(tt.name, func(t *testing.T) {
			if _, err := r.checkMCPUpdateStatus(context.TODO()); (err != nil) != tt.wantErr {
				t.Errorf("checkMCPUpdateStatus() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
