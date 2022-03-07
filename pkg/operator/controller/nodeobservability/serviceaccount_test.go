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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func TestEnsureServiceAccount(t *testing.T) {
	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		expectedSA      corev1.ServiceAccount
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedExist:   true,
			expectedSA: corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: test.TestNamespace,
				},
				Secrets: []corev1.ObjectReference{{
					Name: test.SecretName,
				}},
			},
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceAccountName,
						Namespace: test.TestNamespace,
					},
					Secrets: []corev1.ObjectReference{{
						Name: test.SecretName,
					}},
				},
			},
			expectedExist: true,
			expectedSA: corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: test.TestNamespace,
				},
				Secrets: []corev1.ObjectReference{{
					Name: test.SecretName,
				}},
			},
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
			// ensure secret
			_, secret, err := r.ensureSecret(context.TODO(), nodeObs)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}
			r.Log.Info(fmt.Sprintf("Secret : %s", secret.Name))

			gotExist, _, err := r.ensureServiceAccount(context.TODO(), nodeObs, secret)
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
				t.Errorf("expected service account's exist to be %t, got %t", tc.expectedExist, gotExist)
			}
		})
	}
}
