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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

const (
	TestOperatorNameSpace      = "node-observability-operator"
	TestControllerResourceName = "machineconfig-test"
)

func testReconcileRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      TestControllerResourceName,
			Namespace: TestOperatorNameSpace,
		},
	}
}

func testNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: TestOperatorNameSpace,
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
			Name:      TestControllerResourceName,
			Namespace: TestOperatorNameSpace,
		},
		Spec: v1alpha1.NodeObservabilityMachineConfigSpec{
			EnableCrioProfiling:    true,
			EnableKubeletProfiling: true,
		},
		Status: v1alpha1.NodeObservabilityMachineConfigStatus{
			UpdateStatus: v1alpha1.ConfigUpdateStatus{
				InProgress: corev1.ConditionFalse,
			},
		},
	}
}

func testNodeObsMCToBeDeleted() *v1alpha1.NodeObservabilityMachineConfig {
	mc := testNodeObsMC()
	mc.Finalizers = append(mc.Finalizers, finalizer)
	mc.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	return mc
}

func TestReconcile(t *testing.T) {

	var ctx context.Context

	r := &MachineConfigReconciler{
		Scheme:         test.Scheme,
		Log:            zap.New(zap.UseDevMode(true)),
		EventRecorder:  record.NewFakeRecorder(100),
		PrevSyncChange: make(map[string]PrevSyncData),
	}
	request := testReconcileRequest()

	tests := []struct {
		name    string
		reqObjs []runtime.Object
		wantErr bool
	}{
		{
			name:    "controller resource does not exist",
			reqObjs: []runtime.Object{testNamespace()},
			wantErr: false,
		},
		{
			name:    "controller resource exists",
			reqObjs: []runtime.Object{testNamespace(), testNodeObsMC()},
			wantErr: false,
		},
		{
			name:    "controller resource marked for deletion",
			reqObjs: []runtime.Object{testNamespace(), testNodeObsMCToBeDeleted()},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tt.reqObjs...).Build()
			r.Client = c

			if _, err := r.Reconcile(ctx, request); (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckProfConf(t *testing.T) {

	var ctx context.Context
	nodemc := testNodeObsMC()

	r := &MachineConfigReconciler{
		CtrlConfig:     nodemc,
		Scheme:         test.Scheme,
		Log:            zap.New(zap.UseDevMode(true)),
		EventRecorder:  record.NewFakeRecorder(100),
		PrevSyncChange: make(map[string]PrevSyncData),
	}

	criomc, _ := r.getCrioConfig()
	kubeletmc, _ := r.getKubeletConfig()

	tests := []struct {
		name    string
		reqObjs []runtime.Object
		preReq  func(*MachineConfigReconciler)
		wantErr bool
	}{
		{
			name:    "crio profiling enabled",
			reqObjs: []runtime.Object{nodemc},
			wantErr: false,
		},
		{
			name:    "crio profiling enabled and exists",
			reqObjs: []runtime.Object{nodemc, criomc},
			wantErr: false,
		},
		{
			name:    "crio profiling disabled",
			reqObjs: []runtime.Object{nodemc, criomc},
			preReq: func(r *MachineConfigReconciler) {
				r.CtrlConfig.Spec.EnableCrioProfiling = false
			},
			wantErr: false,
		},
		{
			name:    "kubelet profiling enabled",
			reqObjs: []runtime.Object{nodemc, criomc},
			wantErr: false,
		},
		{
			name:    "kubelet profiling enabled and exists",
			reqObjs: []runtime.Object{nodemc, criomc, kubeletmc},
			wantErr: false,
		},
		{
			name:    "kubelet profiling disabled",
			reqObjs: []runtime.Object{nodemc, criomc, kubeletmc},
			preReq: func(r *MachineConfigReconciler) {
				r.CtrlConfig.Spec.EnableKubeletProfiling = false
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preReq != nil {
				tt.preReq(r)
			}

			c := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tt.reqObjs...).Build()
			r.Client = c

			if err := r.checkProfConf(ctx); (err != nil) != tt.wantErr {
				t.Errorf("checkProfConf() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRevertPrevSyncChanges(t *testing.T) {

	var ctx context.Context

	r := &MachineConfigReconciler{
		Scheme:        test.Scheme,
		Log:           zap.New(zap.UseDevMode(true)),
		EventRecorder: record.NewFakeRecorder(100),
	}

	criomc, _ := r.getCrioConfig()
	kubeletmc, _ := r.getKubeletConfig()

	tests := []struct {
		name         string
		reqObjs      []runtime.Object
		prevSyncData map[string]PrevSyncData
		wantErr      bool
	}{
		{
			name:         "no changes to revert",
			reqObjs:      []runtime.Object{},
			prevSyncData: map[string]PrevSyncData{},
			wantErr:      false,
		},
		{
			name:    "revert crio create",
			reqObjs: []runtime.Object{criomc},
			prevSyncData: map[string]PrevSyncData{
				"crio": {
					action: "created",
					config: *criomc,
				},
			},
			wantErr: false,
		},
		{
			name:    "revert crio delete",
			reqObjs: []runtime.Object{},
			prevSyncData: map[string]PrevSyncData{
				"crio": {
					action: "deleted",
				},
			},
			wantErr: false,
		},
		{
			name:    "revert kubelet create",
			reqObjs: []runtime.Object{kubeletmc},
			prevSyncData: map[string]PrevSyncData{
				"kubelet": {
					action: "created",
					config: *kubeletmc,
				},
			},
			wantErr: false,
		},
		{
			name:    "revert kubelet delete",
			reqObjs: []runtime.Object{},
			prevSyncData: map[string]PrevSyncData{
				"kubelet": {
					action: "deleted",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tt.reqObjs...).Build()
			r.Client = c

			r.PrevSyncChange = tt.prevSyncData

			if err := r.revertPrevSyncChanges(ctx); (err != nil) != tt.wantErr {
				t.Errorf("revertPrevSyncChanges() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
