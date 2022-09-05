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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func TestEnsureServiceAccount(t *testing.T) {
	nodeObs := &operatorv1alpha1.NodeObservability{}
	makeServiceAccount := func() *corev1.ServiceAccount {
		sa := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: nodeObs.Namespace,
			},
		}
		return &sa
	}
	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		expectedSA      *corev1.ServiceAccount
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedSA:      makeServiceAccount(),
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				makeServiceAccount(),
			},
			expectedSA: makeServiceAccount(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := &operatorv1alpha1.NodeObservability{}

			_, err := r.ensureServiceAccount(context.TODO(), nodeObs, test.TestNamespace)
			if err != nil {
				t.Fatalf("unexpected error received: %v", err)
			}

			s := &corev1.ServiceAccount{}
			err = r.Client.Get(context.Background(), types.NamespacedName{Name: tc.expectedSA.Name, Namespace: tc.expectedSA.Namespace}, s)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if s == nil {
				t.Fatalf("expected serviceaccount %s/%s not created", tc.expectedSA.GetNamespace(), tc.expectedSA.GetName())
			}

		})
	}
}
