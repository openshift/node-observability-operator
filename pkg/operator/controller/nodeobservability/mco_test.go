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
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
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
		Spec: v1alpha1.NodeObservabilitySpec{
			Type: v1alpha1.CrioKubeletNodeObservabilityType,
		},
	}

	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		expectedMCO     *v1alpha1.NodeObservabilityMachineConfig
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedMCO:     nomc,
		},
		{
			name: "Exists but needs to be updated",
			existingObjects: []runtime.Object{
				&v1alpha1.NodeObservabilityMachineConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: NodeObservabilityMachineConfigTest,
					},
					Spec: v1alpha1.NodeObservabilityMachineConfigSpec{
						Debug: v1alpha1.NodeObservabilityDebug{
							EnableCrioProfiling: false,
						},
					},
				},
			},
			expectedMCO: nomc,
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

			obj, err := r.ensureNOMC(context.TODO(), nodeObs)
			if err != nil {
				t.Fatalf("unexpected error received: %v", err)
			}

			nomc := &v1alpha1.NodeObservabilityMachineConfig{}
			err = r.Client.Get(context.Background(), types.NamespacedName{Namespace: test.TestNamespace, Name: NodeObservabilityMachineConfigTest}, nomc)
			if err != nil {
				t.Fatalf("failed to get daemonset: %v", err)
			}

			if diff := cmp.Diff(nomc.Spec, tc.expectedMCO.Spec); diff != "" {
				t.Errorf("resource mismatch:\n%s", diff)
			}

			if obj != nil {
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
