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
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/machineconfig/machineconfigfakes"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

const (
	TestControllerResourceName = "machineconfig-test"
)

var (
	testError = fmt.Errorf("test client error")
)

func testReconciler() *MachineConfigReconciler {
	nomc := testNodeObsMC()
	return &MachineConfigReconciler{
		Scheme:        test.Scheme,
		Log:           zap.New(zap.UseDevMode(true)),
		EventRecorder: record.NewFakeRecorder(100),
		CtrlConfig:    nomc,
		Node: NodeSyncData{
			PrevReconcileUpd: make(map[string]LabelInfo),
		},
		MachineConfig: MachineConfigSyncData{
			PrevReconcileUpd: make(map[string]MachineConfigInfo),
		},
	}
}

func testReconcileRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: TestControllerResourceName,
		},
	}
}

func testNodeObsMC() *v1alpha1.NodeObservabilityMachineConfig {
	return &v1alpha1.NodeObservabilityMachineConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NodeObservabilityMachineConfig",
			APIVersion: "nodeobservability.olm.openshift.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: TestControllerResourceName,
		},
		Spec: v1alpha1.NodeObservabilityMachineConfigSpec{
			Debug: v1alpha1.NodeObservabilityDebug{
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
			"machineconfiguration.openshift.io/reason":               Empty,
			"machineconfiguration.openshift.io/state":                "Done",
			"volumes.kubernetes.io/controller-managed-attach-detach": "true",
		},
		Labels: map[string]string{
			"beta.kubernetes.io/arch": "amd64",
			"beta.kubernetes.io/os":   "linux",
			"kubernetes.io/arch":      "amd64",
			"kubernetes.io/os":        "linux",
			WorkerNodeRoleLabelName:   Empty,
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

func testNodeObsNodes() []runtime.Object {
	nodes := testWorkerNodes()

	for _, node := range nodes {
		node.(*corev1.Node).ObjectMeta.Labels[NodeObservabilityNodeRoleLabelName] = Empty
	}

	return nodes
}

func testNodeObsMCP(r *MachineConfigReconciler) *mcv1.MachineConfigPool {
	mcp := r.GetProfilingMCP(ProfilingMCPName)

	mcp.Spec.Configuration.ObjectReference = corev1.ObjectReference{
		Name: "rendered-nodeobservability-9d2d6f47a54e5828cf2917d760b54a99",
	}
	mcp.Status = mcv1.MachineConfigPoolStatus{
		Configuration: mcv1.MachineConfigPoolStatusConfiguration{
			ObjectReference: mcp.Spec.Configuration.ObjectReference,
			Source:          mcp.Spec.Configuration.Source,
		},
		MachineCount:            0,
		UpdatedMachineCount:     0,
		ReadyMachineCount:       0,
		UnavailableMachineCount: 0,
		DegradedMachineCount:    0,
		Conditions: []mcv1.MachineConfigPoolCondition{
			{
				Type:               mcv1.MachineConfigPoolUpdating,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               mcv1.MachineConfigPoolUpdated,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               mcv1.MachineConfigPoolDegraded,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               mcv1.MachineConfigPoolNodeDegraded,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               mcv1.MachineConfigPoolRenderDegraded,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
			},
		},
	}
	return mcp
}

func testWorkerMCP() *mcv1.MachineConfigPool {

	return &mcv1.MachineConfigPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       MCPoolKind,
			APIVersion: MCAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: WorkerNodeMCPName,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/mco-built-in":          Empty,
				"pools.operator.machineconfiguration.openshift.io/worker": Empty,
			},
		},
		Spec: mcv1.MachineConfigPoolSpec{
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{
					Name: "rendered-worker-56630020df0d626345d7fd13172dfd02",
				},
				Source: []corev1.ObjectReference{
					{
						Kind:       MCKind,
						Name:       "00-worker",
						APIVersion: MCAPIVersion,
					},
				},
			},
			MachineConfigSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					MCRoleLabelName: WorkerNodeRoleName,
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					WorkerNodeRoleLabelName: Empty,
				},
			},
		},
		Status: mcv1.MachineConfigPoolStatus{
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{
					Name: "rendered-worker-56630020df0d626345d7fd13172dfd02",
				},
				Source: []corev1.ObjectReference{
					{
						Kind:       MCKind,
						Name:       "00-worker",
						APIVersion: MCAPIVersion,
					},
				},
			},
			MachineCount:            3,
			UpdatedMachineCount:     3,
			ReadyMachineCount:       3,
			UnavailableMachineCount: 0,
			DegradedMachineCount:    0,
			Conditions: []mcv1.MachineConfigPoolCondition{
				{
					Type:               mcv1.MachineConfigPoolUpdating,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
				},
				{
					Type:               mcv1.MachineConfigPoolUpdated,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Message:            "All nodes are updated with rendered-worker-56630020df0d626345d7fd13172dfd02",
				},
				{
					Type:               mcv1.MachineConfigPoolDegraded,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
				},
				{
					Type:               mcv1.MachineConfigPoolNodeDegraded,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
				},
				{
					Type:               mcv1.MachineConfigPoolRenderDegraded,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}
}

func testUpdateMCPCondition(conditions *[]mcv1.MachineConfigPoolCondition,
	condType mcv1.MachineConfigPoolConditionType, status corev1.ConditionStatus) {
	for i := range *conditions {
		if (*conditions)[i].Type == condType {
			(*conditions)[i].Status = status
			return
		}
	}
}

func TestReconcile(t *testing.T) {

	ctx := log.IntoContext(context.TODO(), zap.New(zap.UseDevMode(true)))
	r := testReconciler()
	request := testReconcileRequest()
	nodes := testWorkerNodes()
	labeledNodes := testNodeObsNodes()
	mcp := testNodeObsMCP(r)
	workerMCP := testWorkerMCP()
	criomc, _ := r.getCrioConfig()

	tests := []struct {
		name       string
		reqObjs    []runtime.Object
		preReq     func(*MachineConfigReconciler, *[]runtime.Object)
		asExpected func(v1alpha1.NodeObservabilityMachineConfigStatus, ctrl.Result) bool
		newNOMC    bool
		wantErr    bool
	}{
		{
			name:    "controller resource exists and debug enabled",
			reqObjs: append([]runtime.Object{workerMCP}, nodes...),
			wantErr: false,
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.Node.PrevReconcileUpd["test-worker-4"] = LabelInfo{
					NodeObservabilityNodeRoleLabelName,
					Empty,
					remove,
				}
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource reconcile request too soon, high boundary value",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, labeledNodes...),
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.LastReconcile = metav1.Now()
			},
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter > defaultRequeueTime ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource reconcile request too soon, mid boundary value",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, labeledNodes...),
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.LastReconcile = metav1.Time{Time: metav1.Now().Time.Add(-30 * time.Second)}
			},
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter > 30*time.Second ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource reconcile request too soon, low boundary value",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, labeledNodes...),
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.LastReconcile = metav1.Time{Time: metav1.Now().Time.Add(-defaultRequeueTime)}
			},
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource exists and debug already enabled",
			reqObjs: append([]runtime.Object{criomc, workerMCP}, labeledNodes...),
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				mcp.Status.MachineCount = 3
				mcp.Status.UpdatedMachineCount = 3
				mcp.Status.ReadyMachineCount = 3
				testUpdateMCPCondition(&mcp.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionTrue)
				*o = append(*o, mcp)
			},
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					!status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource exists and debug disabled",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, labeledNodes...),
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Spec.Debug.EnableCrioProfiling = false
				r.Node.PrevReconcileUpd["test-worker-4"] = LabelInfo{
					NodeObservabilityNodeRoleLabelName,
					Empty,
					add,
				}
			},
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource exists and debug already disabled",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, nodes...),
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource marked for deletion",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, labeledNodes...),
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer)
				now := metav1.Now()
				r.CtrlConfig.DeletionTimestamp = &now
			},
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource deletion completed",
			reqObjs: append([]runtime.Object{mcp, criomc, workerMCP}, nodes...),
			wantErr: false,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
		},
		{
			name:    "controller resource does not exist",
			reqObjs: []runtime.Object{},
			newNOMC: true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.CtrlConfig.Status.LastReconcile = metav1.Time{}
			if tt.preReq != nil {
				tt.preReq(r, &tt.reqObjs)
			}
			if !tt.newNOMC {
				tt.reqObjs = append(tt.reqObjs, r.CtrlConfig)
			}

			c := fake.NewClientBuilder().
				WithScheme(test.Scheme).
				WithRuntimeObjects(tt.reqObjs...).
				Build()
			r.impl = &defaultImpl{Client: c}

			result, err := r.Reconcile(ctx, request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() err: %v, wantErr: %v", err, tt.wantErr)
			}

			if tt.asExpected != nil {
				if !tt.asExpected(r.CtrlConfig.Status, result) {
					t.Errorf("Reconcile() result: %+v", result)
					t.Errorf("Reconcile() status: %+v", r.CtrlConfig.Status)
				}
			}
		})
	}
}

func TestMonitorProgress(t *testing.T) {

	ctx := log.IntoContext(context.TODO(), zap.New(zap.UseDevMode(true)))
	r := testReconciler()
	mcp := testNodeObsMCP(r)
	workerMCP := testWorkerMCP()

	tests := []struct {
		name       string
		reqObjs    []runtime.Object
		preReq     func(*MachineConfigReconciler, *[]runtime.Object)
		asExpected func(v1alpha1.NodeObservabilityMachineConfigStatus, ctrl.Result) bool
		wantErr    bool
	}{
		{
			name: "nodeobservability MCP does not exist",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionTrue, v1alpha1.ReasonEnabled, Empty)
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonInProgress, Empty)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "nodeObservability MCP update progressing",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				mcp.Status.MachineCount = 3
				mcp.Status.UpdatedMachineCount = 2
				mcp.Status.ReadyMachineCount = 1
				testUpdateMCPCondition(&mcp.Status.Conditions,
					mcv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
				*o = append(*o, mcp)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name:    "nodeObservability MCP in degraded state",
			reqObjs: []runtime.Object{workerMCP},
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				mcp.Status.MachineCount = 3
				mcp.Status.UpdatedMachineCount = 2
				mcp.Status.ReadyMachineCount = 1
				mcp.Status.DegradedMachineCount = 1
				testUpdateMCPCondition(&mcp.Status.Conditions,
					mcv1.MachineConfigPoolDegraded, corev1.ConditionTrue)
				*o = append(*o, mcp)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					!status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					!status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "worker MCP does not exist",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, Empty)
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonInProgress, Empty)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "worker MCP update progressing",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, Empty)
				workerMCP.Status.MachineCount = 3
				workerMCP.Status.UpdatedMachineCount = 2
				workerMCP.Status.ReadyMachineCount = 1
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionFalse)
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
				*o = append(*o, workerMCP)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "worker MCP in degraded state",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				workerMCP.Status.MachineCount = 3
				workerMCP.Status.UpdatedMachineCount = 2
				workerMCP.Status.ReadyMachineCount = 1
				workerMCP.Status.DegradedMachineCount = 1
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolDegraded, corev1.ConditionTrue)
				*o = append(*o, workerMCP)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "worker MCP update progressing due to a failure",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, Empty)
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, Empty)
				workerMCP = testWorkerMCP()
				workerMCP.Status.MachineCount = 3
				workerMCP.Status.UpdatedMachineCount = 2
				workerMCP.Status.ReadyMachineCount = 1
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
				*o = append(*o, workerMCP)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "worker MCP in degraded state while reverting due to a failure",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				workerMCP.Status.MachineCount = 3
				workerMCP.Status.UpdatedMachineCount = 2
				workerMCP.Status.ReadyMachineCount = 1
				workerMCP.Status.DegradedMachineCount = 1
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolDegraded, corev1.ConditionTrue)
				*o = append(*o, workerMCP)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "worker MCP initializing disabled condition",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, Empty)
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, Empty)
				wmcp := testWorkerMCP()
				wmcp.Status.MachineCount = 0
				wmcp.Status.UpdatedMachineCount = 0
				wmcp.Status.ReadyMachineCount = 0
				testUpdateMCPCondition(&wmcp.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionFalse)
				*o = append(*o, wmcp)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "worker MCP initializing failed condition",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, Empty)
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, Empty)
				wmcp := testWorkerMCP()
				wmcp.Status.MachineCount = 0
				wmcp.Status.UpdatedMachineCount = 0
				wmcp.Status.ReadyMachineCount = 0
				testUpdateMCPCondition(&wmcp.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionFalse)
				*o = append(*o, wmcp)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					!status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preReq != nil {
				tt.preReq(r, &tt.reqObjs)
			}

			c := fake.NewClientBuilder().
				WithScheme(test.Scheme).
				WithRuntimeObjects(tt.reqObjs...).
				Build()
			r.impl = &defaultImpl{Client: c}

			result, err := r.monitorProgress(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("monitorProgress() err: %v, wantErr: %v", err, tt.wantErr)
			}
			if tt.asExpected != nil {
				if !tt.asExpected(r.CtrlConfig.Status, result) {
					t.Errorf("monitorProgress() result: %+v", result)
					t.Errorf("monitorProgress() status: %+v", r.CtrlConfig.Status)
				}
			}
		})
	}
}

func TestReconcileClientFakes(t *testing.T) {

	ctx := log.IntoContext(context.TODO(), zap.New(zap.UseDevMode(true)))
	request := testReconcileRequest()

	tests := []struct {
		name       string
		arg1       context.Context
		arg2       ctrl.Request
		preReq     func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		asExpected func(v1alpha1.NodeObservabilityMachineConfigStatus, ctrl.Result) bool
		wantErr    bool
	}{
		{
			name: "logger init failure",
			arg1: context.Background(),
			arg2: request,
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: false,
		},
		{
			name: "NOMC fetch failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetReturns(testError)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "status update failure when marked for deletion",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == WorkerNodeMCPName {
							mcp = testWorkerMCP()
						}
						if ns.Name == ProfilingMCPName {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						now := metav1.Now()
						nomc.DeletionTimestamp = &now
						nomc.Status.SetCondition(v1alpha1.DebugEnabled,
							metav1.ConditionTrue, v1alpha1.ReasonEnabled, Empty)
						nomc.Status.SetCondition(v1alpha1.DebugReady,
							metav1.ConditionFalse, v1alpha1.ReasonInProgress, Empty)
						nomc.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientStatusUpdateReturns(testError)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "update finalizer failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == WorkerNodeMCPName {
							mcp = testWorkerMCP()
						}
						if ns.Name == ProfilingMCPName {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientUpdateReturns(testError)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != 0 ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "status update failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == WorkerNodeMCPName {
							mcp = testWorkerMCP()
						}
						if ns.Name == ProfilingMCPName {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientStatusUpdateReturns(testError)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					!status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "inspectProfilingMCReq - node patch failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					r.Node.PrevReconcileUpd["test-worker-1"] = LabelInfo{
						NodeObservabilityNodeRoleLabelName,
						Empty,
						add,
					}
					r.Node.PrevReconcileUpd["test-worker-4"] = LabelInfo{
						NodeObservabilityNodeRoleLabelName,
						Empty,
						remove,
					}
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == WorkerNodeMCPName {
							mcp = testWorkerMCP()
						}
						if ns.Name == ProfilingMCPName {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(o)
					case *corev1.Node:
						nodes := testWorkerNodes()
						for _, node := range nodes {
							if ns.Name == "test-worker-2" {
								return kerrors.NewNotFound(corev1.Resource(Empty), ns.Name)
							}
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
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
							if node.GetName() == "test-worker-1" {
								node.ObjectMeta.Labels[NodeObservabilityNodeRoleLabelName] = Empty
							}
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientPatchCalls(func(
					ctx context.Context,
					obj client.Object,
					patch client.Patch,
					opts ...client.PatchOption) error {
					switch obj.(type) {
					case *corev1.Node:
						if obj.GetName() == "test-worker-3" {
							return testError
						}
					}
					return nil
				})
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "inspectProfilingMCReq - mcp create failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						return testError
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(o)
					case *corev1.Node:
						nodes := testWorkerNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientCreateReturns(testError)
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "inspectProfilingMCReq - mc create failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == WorkerNodeMCPName {
							mcp = testWorkerMCP()
						}
						if ns.Name == ProfilingMCPName {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						return testError
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(o)
					case *corev1.Node:
						nodes := testWorkerNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientCreateCalls(func(
					ctx context.Context,
					obj client.Object,
					opts ...client.CreateOption) error {
					switch obj.(type) {
					case *mcv1.MachineConfig:
						return testError
					}
					return nil
				})
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "inspectProfilingMCReq(disable) - node patch failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					r.Node.PrevReconcileUpd["test-worker-1"] = LabelInfo{
						NodeObservabilityNodeRoleLabelName,
						Empty,
						remove,
					}
					r.Node.PrevReconcileUpd["test-worker-4"] = LabelInfo{
						NodeObservabilityNodeRoleLabelName,
						Empty,
						add,
					}
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == WorkerNodeMCPName {
							mcp = testWorkerMCP()
						}
						if ns.Name == ProfilingMCPName {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.Spec.Debug.EnableCrioProfiling = false
						nomc.DeepCopyInto(o)
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == "test-worker-2" {
								return kerrors.NewNotFound(corev1.Resource(Empty), ns.Name)
							}
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testNodeObsNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							if node.GetName() == "test-worker-1" {
								delete(node.ObjectMeta.Labels, NodeObservabilityNodeRoleLabelName)
							}
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientPatchCalls(func(
					ctx context.Context,
					obj client.Object,
					patch client.Patch,
					opts ...client.PatchOption) error {
					switch obj.(type) {
					case *corev1.Node:
						if obj.GetName() == "test-worker-3" {
							return testError
						}
					}
					return nil
				})
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "inspectProfilingMCReq - mcp delete failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testWorkerMCP()
						mcp.Status.MachineCount = 3
						mcp.Status.UpdatedMachineCount = 3
						mcp.Status.ReadyMachineCount = 3
						testUpdateMCPCondition(&mcp.Status.Conditions,
							mcv1.MachineConfigPoolUpdated, corev1.ConditionTrue)
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.Spec.Debug.EnableCrioProfiling = false
						nomc.Status.SetCondition(v1alpha1.DebugReady,
							metav1.ConditionFalse, v1alpha1.ReasonInProgress, Empty)
						nomc.DeepCopyInto(o)
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testNodeObsNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientDeleteCalls(func(
					ctx context.Context,
					obj client.Object,
					opts ...client.DeleteOption) error {
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						return testError
					}
					return nil
				})
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
		{
			name: "inspectProfilingMCReq - mc delete failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *mcv1.MachineConfigPool:
						mcp := testWorkerMCP()
						mcp.Status.MachineCount = 3
						mcp.Status.UpdatedMachineCount = 3
						mcp.Status.ReadyMachineCount = 3
						testUpdateMCPCondition(&mcp.Status.Conditions,
							mcv1.MachineConfigPoolUpdated, corev1.ConditionTrue)
						mcp.DeepCopyInto(o)
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(o)
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.Spec.Debug.EnableCrioProfiling = false
						nomc.Status.SetCondition(v1alpha1.DebugReady,
							metav1.ConditionFalse, v1alpha1.ReasonInProgress, Empty)
						nomc.DeepCopyInto(o)
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch o := list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testNodeObsNodes()
						for i := range nodes {
							node := nodes[i].(*corev1.Node)
							n.Items = append(n.Items, *node)
						}
						n.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientDeleteCalls(func(
					ctx context.Context,
					obj client.Object,
					opts ...client.DeleteOption) error {
					switch obj.(type) {
					case *mcv1.MachineConfig:
						return testError
					}
					return nil
				})
			},
			asExpected: func(status v1alpha1.NodeObservabilityMachineConfigStatus, result ctrl.Result) bool {
				if result.RequeueAfter != defaultRequeueTime ||
					status.IsDebuggingEnabled() ||
					!status.IsMachineConfigInProgress() ||
					status.IsDebuggingFailed() {
					return false
				}
				return true
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &machineconfigfakes.FakeImpl{}
			r := testReconciler()
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.impl = mock

			result, err := r.Reconcile(tt.arg1, tt.arg2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() err: %v, wantErr: %v", err, tt.wantErr)
			}
			if tt.asExpected != nil {
				if !tt.asExpected(r.CtrlConfig.Status, result) {
					t.Errorf("Reconcile() result: %+v", result)
					t.Errorf("Reconcile() status: %+v", r.CtrlConfig.Status)
				}
			}
		})
	}
}

func TestCleanUp(t *testing.T) {

	request := testReconcileRequest()

	tests := []struct {
		name    string
		preReq  func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		wantErr bool
	}{
		{
			name: "ensureProfConfDisabled node listing failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == "test-worker-2" {
								return kerrors.NewNotFound(corev1.Resource(Empty), ns.Name)
							}
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientListReturns(testError)
				m.ClientPatchReturns(testError)
			},
			wantErr: true,
		},
		{
			name: "worker mcp fetch fails",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonInProgress, Empty)
				m.ClientGetReturns(testError)
			},
			wantErr: true,
		},
		{
			name: "debug disabled condition set",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, Empty)
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer)
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						return testError
					case *v1alpha1.NodeObservabilityMachineConfig:
						return testError
					}
					return nil
				})
			},
			wantErr: true,
		},
		{
			name: "debug failed condition set",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, Empty)
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer)
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

			_, err := r.cleanUp(context.TODO(), request)
			if (err != nil) != tt.wantErr {
				t.Errorf("cleanUp() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddFinalizer(t *testing.T) {

	request := testReconcileRequest()

	tests := []struct {
		name       string
		preReq     func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		asExpected func(*v1alpha1.NodeObservabilityMachineConfig, error) bool
	}{
		{
			name: "nomc fetch fails",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer)
				m.ClientGetReturns(testError)
			},
			asExpected: func(nomc *v1alpha1.NodeObservabilityMachineConfig, err error) bool {
				return err != nil
			},
		},
		{
			name: "finalizer already present",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer)
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.NodeObservabilityMachineConfig:
						r.CtrlConfig.DeepCopyInto(o)
					}
					return nil
				})
			},
			asExpected: func(nomc *v1alpha1.NodeObservabilityMachineConfig, err error) bool {
				if err != nil || !hasFinalizer(nomc) {
					return false
				}
				return true
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

			nomc, err := r.addFinalizer(context.TODO(), request)
			if tt.asExpected != nil {
				if !tt.asExpected(nomc, err) {
					t.Errorf("addFinalizer() nomc: %+v", nomc)
					t.Errorf("addFinalizer() error: %+v", err)
				}
			}
		})
	}
}

func TestRemoveFinalizer(t *testing.T) {

	request := testReconcileRequest()

	tests := []struct {
		name       string
		preReq     func(*MachineConfigReconciler, *machineconfigfakes.FakeImpl)
		asExpected func(*v1alpha1.NodeObservabilityMachineConfig, error) bool
	}{
		{
			name: "finalizer not present",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.NodeObservabilityMachineConfig:
						r.CtrlConfig.DeepCopyInto(o)
					}
					return nil
				})
			},
			asExpected: func(nomc *v1alpha1.NodeObservabilityMachineConfig, err error) bool {
				return err == nil
			},
		},
		{
			name: "remove finalizer success",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer, "test")
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.NodeObservabilityMachineConfig:
						r.CtrlConfig.DeepCopyInto(o)
					}
					return nil
				})
			},
			asExpected: func(nomc *v1alpha1.NodeObservabilityMachineConfig, err error) bool {
				if err != nil || hasFinalizer(nomc) {
					return false
				}
				return true
			},
		},
		{
			name: "remove finalizer fails",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.Finalizers = append(r.CtrlConfig.Finalizers, finalizer)
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.NodeObservabilityMachineConfig:
						r.CtrlConfig.DeepCopyInto(o)
					}
					return nil
				})
				m.ClientUpdateReturns(testError)
			},
			asExpected: func(nomc *v1alpha1.NodeObservabilityMachineConfig, err error) bool {
				return err != nil
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

			nomc, err := r.removeFinalizer(context.TODO(), request, finalizer)
			if tt.asExpected != nil {
				if !tt.asExpected(nomc, err) {
					t.Errorf("removeFinalizer() nomc: %+v", nomc)
					t.Errorf("removeFinalizer() error: %+v", err)
				}
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

			err := r.updateStatus(context.TODO())
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
				r.Node.PrevReconcileUpd["test-worker-1"] = LabelInfo{
					NodeObservabilityNodeRoleLabelName,
					Empty,
					add,
				}
				m.ClientGetReturns(testError)
				m.ClientListReturns(testError)
			},
			requeue: true,
			wantErr: true,
		},
		{
			name: "revertNodeLabeling failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.Node.PrevReconcileUpd["test-worker-1"] = LabelInfo{
					NodeObservabilityNodeRoleLabelName,
					Empty,
					add,
				}
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientListReturns(testError)
				m.ClientPatchReturns(testError)
			},
			requeue: true,
			wantErr: true,
		},
		{
			name: "ensureReqMCPExists failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.SetNamespace("test")
			},
			requeue: false,
			wantErr: true,
		},
		{
			name: "ensureReqMCExists failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.CtrlConfig.SetNamespace("test")
			},
			requeue: false,
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

			requeue, err := r.ensureProfConfEnabled(context.TODO())
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureProfConfEnabled() err: %v, wantErr: %v", err, tt.wantErr)
			}
			if requeue != tt.requeue {
				t.Errorf("ensureProfConfEnabled() requeue: %v, wantRequeue: %v", requeue, tt.requeue)
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
				r.Node.PrevReconcileUpd["test-worker-1"] = LabelInfo{
					NodeObservabilityNodeRoleLabelName,
					Empty,
					remove,
				}
				m.ClientGetReturns(testError)
				m.ClientListReturns(testError)
			},
			requeue: true,
			wantErr: true,
		},
		{
			name: "revertNodeUnlabeling failure",
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				r.Node.PrevReconcileUpd["test-worker-1"] = LabelInfo{
					NodeObservabilityNodeRoleLabelName,
					Empty,
					remove,
				}
				m.ClientGetCalls(func(
					ctx context.Context,
					ns types.NamespacedName,
					obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(o)
							}
						}
					}
					return nil
				})
				m.ClientListReturns(testError)
				m.ClientPatchReturns(testError)
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

			requeue, err := r.ensureProfConfDisabled(context.TODO())
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureProfConfDisabled() err: %v, wantErr: %v", err, tt.wantErr)
			}
			if requeue != tt.requeue {
				t.Errorf("ensureProfConfDisabled() requeue: %v, wantRequeue: %v", requeue, tt.requeue)
			}
		})
	}
}

func TestNewPatch(t *testing.T) {

	tests := []struct {
		name       string
		arg        map[string]interface{}
		wantResult bool
		wantErr    bool
	}{
		{
			name:       "bad patch data",
			arg:        nil,
			wantResult: false,
			wantErr:    true,
		},
		{
			name:       "invalid patch operation type",
			arg:        map[string]interface{}{},
			wantResult: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := newPatch(99, "test", tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPatch() err: %v, wantErr: %v", err, tt.wantErr)
			}
			if (result != nil) != tt.wantResult {
				t.Errorf("newPatch() err: %v, wantErr: %v", result, tt.wantResult)
			}
		})
	}
}
