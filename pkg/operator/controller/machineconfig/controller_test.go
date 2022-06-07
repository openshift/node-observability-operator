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
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/machineconfig/machineconfigfakes"
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
			"machineconfiguration.openshift.io/reason":               "",
			"machineconfiguration.openshift.io/state":                "Done",
			"volumes.kubernetes.io/controller-managed-attach-detach": "true",
		},
		Labels: map[string]string{
			"beta.kubernetes.io/arch":        "amd64",
			"beta.kubernetes.io/os":          "linux",
			"kubernetes.io/arch":             "amd64",
			"kubernetes.io/os":               "linux",
			"node-role.kubernetes.io/worker": "",
		},
	}

	nodes := make([]runtime.Object, 0, len(workerNodes))
	for _, name := range workerNodes {
		objMeta.Name = name
		objMeta.Labels["kubernetes.io/hostname"] = name
		objMeta.Annotations["machine.openshift.io/machine"] = fmt.Sprintf("openshift-machine-api/%s", name)
		node := corev1.Node{
			TypeMeta:   typeMeta,
			ObjectMeta: objMeta,
		}
		nodes = append(nodes, &node)
	}

	return nodes
}

func testNodeObsNodes() []runtime.Object {
	nodes := testWorkerNodes()

	for _, node := range nodes {
		node.(*corev1.Node).ObjectMeta.Labels["node-role.kubernetes.io/nodeobservability"] = ""
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
			Kind:       "MachineConfigPool",
			APIVersion: "machineconfiguration.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
			Labels: map[string]string{
				"machineconfiguration.openshift.io/mco-built-in":          "",
				"pools.operator.machineconfiguration.openshift.io/worker": "",
			},
		},
		Spec: mcv1.MachineConfigPoolSpec{
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{
					Name: "rendered-worker-56630020df0d626345d7fd13172dfd02",
				},
				Source: []corev1.ObjectReference{
					{
						Kind:       "MachineConfig",
						Name:       "00-worker",
						APIVersion: "machineconfiguration.openshift.io/v1",
					},
				},
			},
			MachineConfigSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"machineconfiguration.openshift.io/role": "worker",
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/worker": "",
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
						Kind:       "MachineConfig",
						Name:       "00-worker",
						APIVersion: "machineconfiguration.openshift.io/v1",
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
		name    string
		reqObjs []runtime.Object
		preReq  func(*MachineConfigReconciler, *[]runtime.Object)
		wantErr bool
	}{
		{
			name: "nodeobservability MCP does not exist",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionTrue, v1alpha1.ReasonEnabled, "")
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonInProgress, "")
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
			wantErr: false,
		},
		{
			name: "worker MCP does not exist",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, "")
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonInProgress, "")
			},
			wantErr: true,
		},
		{
			name: "worker MCP update progressing",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, "")
				workerMCP.Status.MachineCount = 3
				workerMCP.Status.UpdatedMachineCount = 2
				workerMCP.Status.ReadyMachineCount = 1
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionFalse)
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
				*o = append(*o, workerMCP)
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
			wantErr: false,
		},
		{
			name: "worker MCP update progressing due to a failure",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, "")
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, "")
				workerMCP = testWorkerMCP()
				workerMCP.Status.MachineCount = 3
				workerMCP.Status.UpdatedMachineCount = 2
				workerMCP.Status.ReadyMachineCount = 1
				testUpdateMCPCondition(&workerMCP.Status.Conditions,
					mcv1.MachineConfigPoolUpdating, corev1.ConditionTrue)
				*o = append(*o, workerMCP)
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
			wantErr: false,
		},
		{
			name: "worker MCP initializing disabled condition",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, "")
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonDisabled, "")
				wmcp := testWorkerMCP()
				wmcp.Status.MachineCount = 0
				wmcp.Status.UpdatedMachineCount = 0
				wmcp.Status.ReadyMachineCount = 0
				testUpdateMCPCondition(&wmcp.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionFalse)
				*o = append(*o, wmcp)
			},
			wantErr: false,
		},
		{
			name: "worker MCP initializing failed condition",
			preReq: func(r *MachineConfigReconciler, o *[]runtime.Object) {
				r.CtrlConfig.Status = v1alpha1.NodeObservabilityMachineConfigStatus{}
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugEnabled,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, "")
				r.CtrlConfig.Status.SetCondition(v1alpha1.DebugReady,
					metav1.ConditionFalse, v1alpha1.ReasonFailed, "")
				wmcp := testWorkerMCP()
				wmcp.Status.MachineCount = 0
				wmcp.Status.UpdatedMachineCount = 0
				wmcp.Status.ReadyMachineCount = 0
				testUpdateMCPCondition(&wmcp.Status.Conditions,
					mcv1.MachineConfigPoolUpdated, corev1.ConditionFalse)
				*o = append(*o, wmcp)
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

			_, err := r.monitorProgress(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("monitorProgress() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func TestReconcileNegativeScenarios(t *testing.T) {

	ctx := log.IntoContext(context.TODO(), zap.New(zap.UseDevMode(true)))
	r := testReconciler()
	request := testReconcileRequest()

	tests := []struct {
		name       string
		arg1       context.Context
		arg2       ctrl.Request
		reqObjs    []runtime.Object
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
			wantErr: true,
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
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						if ns.Name == "worker" {
							mcp := testWorkerMCP()
							mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
						}
						if ns.Name == "nodeobservability" {
							mcp := testNodeObsMCP(r)
							mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
						}
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(obj.(*mcv1.MachineConfig))
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						now := metav1.Now()
						nomc.DeletionTimestamp = &now
						nomc.Status.SetCondition(v1alpha1.DebugEnabled,
							metav1.ConditionTrue, v1alpha1.ReasonEnabled, "")
						nomc.Status.SetCondition(v1alpha1.DebugReady,
							metav1.ConditionFalse, v1alpha1.ReasonInProgress, "")
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
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
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						if ns.Name == "worker" {
							mcp := testWorkerMCP()
							mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
						}
						if ns.Name == "nodeobservability" {
							mcp := testNodeObsMCP(r)
							mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
						}
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(obj.(*mcv1.MachineConfig))
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
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
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						if ns.Name == "worker" {
							mcp := testWorkerMCP()
							mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
						}
						if ns.Name == "nodeobservability" {
							mcp := testNodeObsMCP(r)
							mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
						}
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(obj.(*mcv1.MachineConfig))
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
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
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == "worker" {
							mcp = testWorkerMCP()
						}
						if ns.Name == "nodeobservability" {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(obj.(*mcv1.MachineConfig))
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
					case *corev1.Node:
						nodes := testWorkerNodes()
						for _, node := range nodes {
							if ns.Name == "test-worker-2" {
								return kerrors.NewNotFound(corev1.Resource(""), ns.Name)
							}
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(obj.(*corev1.Node))
							}
						}
					}
					return nil
				})
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testWorkerNodes()
						n.Items = make([]corev1.Node, 0, len(nodes))
						for _, node := range nodes {
							n.Items = append(n.Items, *node.(*corev1.Node))
						}
						n.DeepCopyInto(list.(*corev1.NodeList))
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
			wantErr: false,
		},
		{
			name: "inspectProfilingMCReq - mcp create failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						return testError
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(obj.(*mcv1.MachineConfig))
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
					case *corev1.Node:
						nodes := testWorkerNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(obj.(*corev1.Node))
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
					case *mcv1.MachineConfigPool:
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
			name: "inspectProfilingMCReq - mc create failure",
			arg1: ctx,
			arg2: request,
			preReq: func(r *MachineConfigReconciler, m *machineconfigfakes.FakeImpl) {
				m.ClientGetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == "worker" {
							mcp = testWorkerMCP()
						}
						if ns.Name == "nodeobservability" {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
					case *mcv1.MachineConfig:
						return testError
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
					case *corev1.Node:
						nodes := testWorkerNodes()
						for _, node := range nodes {
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(obj.(*corev1.Node))
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
					switch obj.(type) {
					case *mcv1.MachineConfigPool:
						var mcp *mcv1.MachineConfigPool
						if ns.Name == "worker" {
							mcp = testWorkerMCP()
						}
						if ns.Name == "nodeobservability" {
							mcp = testNodeObsMCP(r)
						}
						mcp.DeepCopyInto(obj.(*mcv1.MachineConfigPool))
					case *mcv1.MachineConfig:
						mc, _ := r.getCrioConfig()
						mc.DeepCopyInto(obj.(*mcv1.MachineConfig))
					case *v1alpha1.NodeObservabilityMachineConfig:
						nomc := testNodeObsMC()
						nomc.Spec.Debug.EnableCrioProfiling = false
						nomc.DeepCopyInto(obj.(*v1alpha1.NodeObservabilityMachineConfig))
					case *corev1.Node:
						nodes := testNodeObsNodes()
						for _, node := range nodes {
							if ns.Name == "test-worker-2" {
								return kerrors.NewNotFound(corev1.Resource(""), ns.Name)
							}
							if ns.Name == node.(*corev1.Node).GetName() {
								node.(*corev1.Node).DeepCopyInto(obj.(*corev1.Node))
							}
						}
					}
					return nil
				})
				m.ClientListCalls(func(
					ctx context.Context,
					list client.ObjectList,
					opts ...client.ListOption) error {
					switch list.(type) {
					case *corev1.NodeList:
						n := &corev1.NodeList{}
						nodes := testNodeObsNodes()
						n.Items = make([]corev1.Node, 0, len(nodes))
						for _, node := range nodes {
							n.Items = append(n.Items, *node.(*corev1.Node))
						}
						n.DeepCopyInto(list.(*corev1.NodeList))
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &machineconfigfakes.FakeImpl{}
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
