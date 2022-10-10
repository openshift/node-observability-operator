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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha2"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/machineconfig/machineconfigfakes"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/utils/test"
)

const (
	TestControllerResourceName = "machineconfig-test"
)

var (
	testError = fmt.Errorf("test client error")
)

func testReconciler() *MachineConfigReconciler {
	return &MachineConfigReconciler{
		Scheme:        test.Scheme,
		Log:           zap.New(zap.UseDevMode(true)),
		EventRecorder: record.NewFakeRecorder(100),
	}
}

func testNodeObsMC() *v1alpha2.NodeObservabilityMachineConfig {
	return &v1alpha2.NodeObservabilityMachineConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NodeObservabilityMachineConfig",
			APIVersion: "nodeobservability.olm.openshift.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: TestControllerResourceName,
		},
		Spec: v1alpha2.NodeObservabilityMachineConfigSpec{
			NodeSelector: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			Debug: v1alpha2.NodeObservabilityDebug{
				EnableCrioProfiling: true,
			},
		},
	}
}

func testWorkerNodes() []runtime.Object {
	workerNodes := []string{"test-worker-1", "test-worker-2", "test-worker-3"}

	typeMeta := metav1.TypeMeta{
		Kind:       "Node",
		APIVersion: "v1",
	}
	objMeta := metav1.ObjectMeta{
		Annotations: map[string]string{
			"machineconfiguration.openshift.io/controlPlaneTopology": "HighlyAvailable",
			"machineconfiguration.openshift.io/currentConfig":        "rendered-worker-56630020df0d626345d7fd13172dfd02",
			"machineconfiguration.openshift.io/desiredConfig":        "rendered-worker-56630020df0d626345d7fd13172dfd02",
			"machineconfiguration.openshift.io/reason":               empty,
			"machineconfiguration.openshift.io/state":                "Done",
			"volumes.kubernetes.io/controller-managed-attach-detach": "true",
		},
		Labels: map[string]string{
			"beta.kubernetes.io/arch": "amd64",
			"beta.kubernetes.io/os":   "linux",
			"kubernetes.io/arch":      "amd64",
			"kubernetes.io/os":        "linux",
			WorkerNodeRoleLabelName:   empty,
		},
	}

	nodes := make([]runtime.Object, 0, len(workerNodes))
	for _, name := range workerNodes {
		node := &corev1.Node{
			TypeMeta: typeMeta,
		}
		(&objMeta).DeepCopyInto(&node.ObjectMeta)
		node.ObjectMeta.Name = name
		node.ObjectMeta.Labels["kubernetes.io/hostname"] = name
		node.ObjectMeta.Annotations["machine.openshift.io/machine"] = fmt.Sprintf("openshift-machine-api/%s", name)
		nodes = append(nodes, node)
	}

	return nodes
}

func testMachineConfigPoolWithConditions(name string, mcCount, updatedCount, readyCount, unavailableCount, degradedCount int32, cond []mcv1.MachineConfigPoolCondition) *mcv1.MachineConfigPool {
	return &mcv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: mcv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
		},
		Status: mcv1.MachineConfigPoolStatus{
			MachineCount:            mcCount,
			UpdatedMachineCount:     updatedCount,
			ReadyMachineCount:       readyCount,
			UnavailableMachineCount: unavailableCount,
			DegradedMachineCount:    degradedCount,
			Conditions:              cond,
		},
	}
}

// TODO: add tests for syncNodeObservabilityMCP

func TestCleanUp(t *testing.T) {

	tests := []struct {
		name               string
		preReq             func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		wantErr            bool
		requeue            bool
		expectedConditions []metav1.Condition
	}{
		{
			name: "ensureProfConfDisabled node listing failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientListReturns(testError)
			},
			wantErr: true,
			requeue: false,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonInProgress,
				},
			},
		},
		{
			name: "worker mcp fetch fails",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetReturns(testError)
			},
			wantErr: true,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
				{
					Type:   v1alpha2.DebugReady,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonInProgress,
				},
			},
		},
		{
			name: "debug disabled but waiting for debug ready condition due to mcp update in progress",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testMachineConfigPoolWithConditions("worker", 3, 2, 2, 0, 0, []mcv1.MachineConfigPoolCondition{
							{
								Type:   mcv1.MachineConfigPoolUpdating,
								Status: corev1.ConditionStatus(metav1.ConditionTrue),
							},
						})
						mcp.DeepCopyInto(o)
					}
					return nil
				})
			},
			wantErr: false,
			requeue: true,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
				{
					Type:   v1alpha2.DebugReady,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonInProgress,
				},
			},
		},
		{
			name: "debug disabled but waiting for debug ready condition due to degraded machine count",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testMachineConfigPoolWithConditions("worker", 3, 2, 2, 1, 1, []mcv1.MachineConfigPoolCondition{
							{
								Type:   mcv1.MachineConfigPoolDegraded,
								Status: corev1.ConditionStatus(metav1.ConditionTrue),
							},
						})
						mcp.DeepCopyInto(o)
					}
					return nil
				})
			},
			wantErr: false,
			requeue: true,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
				{
					Type:   v1alpha2.DebugReady,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonInProgress,
				},
			},
		},
		{
			name: "debug disabled but waiting for debug ready condition due to unknown condition",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testMachineConfigPoolWithConditions("worker", 3, 2, 2, 1, 1, []mcv1.MachineConfigPoolCondition{
							{
								Type:   "random-condition",
								Status: corev1.ConditionStatus(metav1.ConditionTrue),
							},
						})
						mcp.DeepCopyInto(o)
					}
					return nil
				})
			},
			wantErr: false,
			requeue: true,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
				{
					Type:   v1alpha2.DebugReady,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonInProgress,
				},
			},
		},
		{
			name: "debug disabled and waiting for debug ready to be disabled because mcp doesn't have required machines",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testMachineConfigPoolWithConditions("worker", 3, 2, 2, 0, 0, []mcv1.MachineConfigPoolCondition{
							{
								Type:   mcv1.MachineConfigPoolUpdated,
								Status: corev1.ConditionStatus(metav1.ConditionTrue),
							},
						})
						mcp.DeepCopyInto(o)
					}
					return nil
				})
			},
			wantErr: false,
			requeue: true,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
				{
					Type:   v1alpha2.DebugReady,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonInProgress,
				},
			},
		},
		{
			name: "debug disabled and debug ready disabled",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testWorkerNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							node.ObjectMeta.Labels[NodeObservabilityNodeRoleLabelName] = empty
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testMachineConfigPoolWithConditions("worker", 3, 3, 3, 0, 0, []mcv1.MachineConfigPoolCondition{
							{
								Type:   mcv1.MachineConfigPoolUpdated,
								Status: corev1.ConditionStatus(metav1.ConditionTrue),
							},
						})
						mcp.DeepCopyInto(o)
					}
					return nil
				})
			},
			wantErr: false,
			requeue: false,
			expectedConditions: []metav1.Condition{
				{
					Type:   v1alpha2.DebugEnabled,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
				{
					Type:   v1alpha2.DebugReady,
					Status: metav1.ConditionFalse,
					Reason: v1alpha2.ReasonDisabled,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler()
			mock := &machineconfigfakes.FakeImpl{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.impl = mock

			nomc := testNodeObsMC()
			requeue, err := r.cleanUp(context.TODO(), nomc)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("cleanUp() err: %v, wantErr: %v", err, tt.wantErr)
				} else {
					for _, c1 := range nomc.Status.Conditions {
						for _, c2 := range tt.expectedConditions {
							if c1.Type == c2.Type {
								if c1.Status != c2.Status || c1.Reason != c2.Reason {
									t.Errorf("failed expected %+v but got %+v condition on nomc", c2, c1)
								}
							}
						}
					}
				}
			}

			if tt.requeue != requeue {
				t.Errorf("expected requeue=%v for cleanup but got %v", tt.requeue, requeue)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {

	tests := []struct {
		name    string
		preReq  func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		wantErr bool
	}{
		{
			name: "status update fails with nomc fetch error",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetReturns(testError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler()
			mock := &machineconfigfakes.FakeImpl{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.impl = mock

			err := r.updateStatus(context.TODO(), &v1alpha2.NodeObservabilityMachineConfig{})
			if (err != nil) != tt.wantErr {
				t.Errorf("updateStatus() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureProfConfEnabled(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		requeue bool
		wantErr bool
	}{
		{
			name: "ensureReqNodeLabelExists failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetReturns(testError)
				m.ClientListReturns(testError)
			},
			requeue: true,
			wantErr: true,
		},
		{
			name: "fails while enabling CRI-O config",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testWorkerNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientCreateCalls(func(ctx context.Context, o client.Object, co ...client.CreateOption) error {
					switch o.(type) {
					case *mcv1.MachineConfig:
						return testError
					case *mcv1.MachineConfigPool:
						return nil
					}
					return nil
				})
			},
			requeue: false,
			wantErr: true,
		},
		{
			name: "fails while creating mcp",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testWorkerNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							node.ObjectMeta.Labels[NodeObservabilityNodeRoleLabelName] = empty
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientCreateCalls(func(ctx context.Context, o client.Object, co ...client.CreateOption) error {
					switch o.(type) {
					case *mcv1.MachineConfig:
						return nil
					case *mcv1.MachineConfigPool:
						return testError
					}
					return nil
				})
			},
			requeue: false,
			wantErr: true,
		},
		{
			name: "successfully create crio mc",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testWorkerNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							node.ObjectMeta.Labels[NodeObservabilityNodeRoleLabelName] = empty
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientCreateCalls(func(ctx context.Context, o client.Object, co ...client.CreateOption) error {
					return nil
				})
			},
			requeue: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler()
			mock := &machineconfigfakes.FakeImpl{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.impl = mock
			nomc := testNodeObsMC()
			err := r.ensureProfConfEnabled(context.TODO(), nomc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureProfConfEnabled() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureProfConfDisabled(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		requeue bool
		wantErr bool
	}{
		{
			name: "ensureReqNodeLabelNotExists failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetReturns(testError)
				m.ClientListReturns(testError)
			},
			requeue: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler()
			mock := &machineconfigfakes.FakeImpl{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.impl = mock

			err := r.ensureProfConfDisabled(context.TODO(), &v1alpha2.NodeObservabilityMachineConfig{})
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureProfConfDisabled() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewPatch(t *testing.T) {
	tests := []struct {
		name    string
		arg     map[string]interface{}
		preReq  func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		action  string
		wantErr bool
	}{
		{
			name:    "bad patch data",
			arg:     nil,
			action:  "add",
			preReq:  func(mcr *MachineConfigReconciler, fi *machineconfigfakes.FakeImpl) {},
			wantErr: true,
		},
		{
			name:   "invalid patch operation type",
			arg:    map[string]interface{}{},
			action: "dummy-action",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientPatchReturns(testError)
			},
			wantErr: true,
		},
	}

	r := testReconciler()
	mock := &machineconfigfakes.FakeImpl{}
	for _, tt := range tests {
		if tt.preReq != nil {
			tt.preReq(r, mock)
		}
		r.impl = mock

		t.Run(tt.name, func(t *testing.T) {
			err := r.patchNodeLabels(context.Background(), &corev1.Node{}, tt.action, "test", tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPatch() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}
