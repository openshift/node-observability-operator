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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func makeClusterRoleBinding(subjects []rbacv1.Subject, roleref rbacv1.RoleRef) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
		},
		Subjects: subjects,
		RoleRef:  roleref,
	}
	return &clusterRoleBinding
}

func TestDeleteClusterRole(t *testing.T) {
	testCasesClusterRoleBinding := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			errExpected:     false,
			expectedExist:   false,
		},
		{
			name:            "Exists",
			existingObjects: []runtime.Object{
				// makeClusterRoleBinding(),
			},
			expectedExist: false,
			errExpected:   false,
		},
	}

	for _, tc := range testCasesClusterRoleBinding {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := testNodeObservability()
			err := r.deleteClusterRoleBinding(nodeObs)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}
			if tc.errExpected {
				t.Fatalf("Error expected but wasn't received")
			}
			name := types.NamespacedName{
				Namespace: nodeObs.Namespace,
				Name:      clusterRoleBindingName,
			}
			err = cl.Get(context.TODO(), name, &rbacv1.ClusterRoleBinding{})
			gotExist := true
			if errors.IsNotFound(err) {
				gotExist = false
			} else if !tc.errExpected {
				t.Fatalf("unexpected error received: %v", err)
			}
			if gotExist != tc.expectedExist {
				t.Errorf("expected clusterrolebinding's exist to be %t, got %t", tc.expectedExist, gotExist)
			}
		})
	}
}

func TestEnsureClusterRole(t *testing.T) {
	testCasesClusterRoleBinding := []struct {
		name            string
		existingObjects []runtime.Object
		expectedCRB     *rbacv1.ClusterRoleBinding
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedCRB: makeClusterRoleBinding([]rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccountName,
					Namespace: test.TestNamespace,
				},
			},
				rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterRoleName,
				}),
		},
		{
			name: "Outdated clusterrolebinding exists",
			existingObjects: []runtime.Object{
				makeClusterRoleBinding([]rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "oldsaname",
						Namespace: test.TestNamespace,
					},
				},
					rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "dummyclusterrole",
					}),
			},
			expectedCRB: makeClusterRoleBinding([]rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccountName,
					Namespace: test.TestNamespace,
				},
			},
				rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterRoleName,
				}),
		},
	}

	for _, tc := range testCasesClusterRoleBinding {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := &operatorv1alpha2.NodeObservability{}

			crb, err := r.ensureClusterRoleBinding(context.TODO(), nodeObs, serviceAccountName, test.TestNamespace)
			if err != nil {
				t.Fatalf("unexpected error received: %v", err)
			}

			opts := cmpopts.EquateEmpty()
			if diff := cmp.Diff(tc.expectedCRB.Subjects, crb.Subjects, opts); diff != "" {
				t.Fatalf("clusterrolebinding subjects mismatch: \n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedCRB.RoleRef, crb.RoleRef, opts); diff != "" {
				t.Fatalf("clusterrolebinding roleref mismatch: \n%s", diff)
			}
		})
	}
}
