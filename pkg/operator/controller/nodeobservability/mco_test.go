/*
Copyright 2021.

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

package nodeobservabilitycontroller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

const (
	NodeObservabilityMachineConfigTest = "NodeObservabilityMachineConfig-test"
)

func TestEnsureMCO(t *testing.T) {
	nomc := &v1alpha1.NodeObservabilityMachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: NodeObservabilityMachineConfigTest,
		},
		Spec: v1alpha1.NodeObservabilityMachineConfigSpec{
			Debug: v1alpha1.NodeObservabilityDebug{
				EnableCrioProfiling: true,
			},
		},
	}
	nodeObs := &v1alpha1.NodeObservability{
		ObjectMeta: metav1.ObjectMeta{Name: NodeObservabilityMachineConfigTest},
	}

	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		expectedMCO     *v1alpha1.NodeObservabilityMachineConfig
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedExist:   true,
			expectedMCO:     &v1alpha1.NodeObservabilityMachineConfig{},
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				nomc,
			},
			expectedExist: true,
			expectedMCO:   nomc,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}

			gotExist, obj, err := r.ensureNOMC(context.TODO(), nodeObs)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}

			if tc.errExpected {
				t.Fatalf("Error expected but wasn't received")
			}
			if gotExist != tc.expectedExist {
				t.Errorf("expected machineconfig exist to be %t, got %t", tc.expectedExist, gotExist)
			}
			if gotExist {
				for _, ref := range obj.GetOwnerReferences() {
					if ref.Name == NodeObservabilityMachineConfigTest {
						return
					}
				}
				t.Errorf("expected OwnerReference to point to: %s", NodeObservabilityMachineConfigTest)
			}
		})
	}
}
