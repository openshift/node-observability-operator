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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func makeClusterRole() *rbacv1.ClusterRole {
	nodeObs := &operatorv1alpha1.NodeObservability{}
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleName,
			Namespace: nodeObs.Namespace,
			Labels:    labelsForClusterRole(clusterRoleName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{secGroup},
				Resources:     []string{secResource},
				ResourceNames: []string{secResourceName},
				Verbs:         []string{use},
			},
			{
				Verbs:     []string{get, list},
				APIGroups: []string{""},
				Resources: []string{nodes, nodesProxy, pods},
			},
			{
				Verbs:           []string{get},
				NonResourceURLs: []string{urlStatus, urlPprof},
			},
		},
	}
	return &clusterRole
}

func makeClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      serviceAccount,
				Name:      serviceAccountName,
				Namespace: test.TestNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     clusterRole,
			Name:     clusterRoleName,
			APIGroup: apiGroup,
		},
	}
	return &clusterRoleBinding
}
func TestDeleteClusterRole(t *testing.T) {
	testCasesClusterRole := []struct {
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
			name: "Exists",
			existingObjects: []runtime.Object{
				makeClusterRole(),
			},
			expectedExist: false,
			errExpected:   false,
		},
	}
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
			name: "Exists",
			existingObjects: []runtime.Object{
				makeClusterRoleBinding(),
			},
			expectedExist: false,
			errExpected:   false,
		},
	}

	for _, tc := range testCasesClusterRole {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := testNodeObservability()
			err := r.deleteClusterRole(nodeObs)
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
				Name:      clusterRoleName,
			}
			err = cl.Get(context.TODO(), name, &rbacv1.ClusterRole{})
			gotExist := true
			if errors.IsNotFound(err) {
				gotExist = false
			} else if !tc.errExpected {
				t.Fatalf("unexpected error received: %v", err)
			}
			if gotExist != tc.expectedExist {
				t.Errorf("expected clusterrole's exist to be %t, got %t", tc.expectedExist, gotExist)
			}
		})
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

	testCasesClusterRole := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		expectedCR      *rbacv1.ClusterRole
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedExist:   true,
			expectedCR:      makeClusterRole(),
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				makeClusterRole(),
			},
			expectedExist: true,
			expectedCR:    makeClusterRole(),
		},
	}
	testCasesClusterRoleBinding := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		expectedCRB     *rbacv1.ClusterRoleBinding
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedExist:   true,
			expectedCRB:     makeClusterRoleBinding(),
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				makeClusterRoleBinding(),
			},
			expectedExist: true,
			expectedCRB:   makeClusterRoleBinding(),
		},
	}

	for _, tc := range testCasesClusterRole {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := &operatorv1alpha1.NodeObservability{}
			gotExist, _, err := r.ensureClusterRole(context.TODO(), nodeObs)
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
	for _, tc := range testCasesClusterRoleBinding {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := &operatorv1alpha1.NodeObservability{}
			serviceAccount, err := r.ensureServiceAccount(context.TODO(), nodeObs, test.TestNamespace)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}
			r.Log.Info(fmt.Sprintf("ServiceAccount : %s", serviceAccount.Name))

			gotExist, _, err := r.ensureClusterRoleBinding(context.TODO(), nodeObs, serviceAccount.Name, test.TestNamespace)
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
