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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	securityv1 "github.com/openshift/api/security/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	test "github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func makeScc() *securityv1.SecurityContextConstraints {
	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            sccName,
			ResourceVersion: "1",
		},
		Priority:                 nil,
		AllowPrivilegedContainer: true,
		DefaultAddCapabilities:   nil,
		RequiredDropCapabilities: []corev1.Capability{"MKNOD"},
		AllowedCapabilities:      nil,
		AllowHostDirVolumePlugin: true,
		Volumes:                  []securityv1.FSType{securityv1.FSTypeHostPath, securityv1.FSTypeSecret, securityv1.FSTypeConfigMap},
		AllowHostNetwork:         false,
		AllowHostPorts:           false,
		AllowHostPID:             false,
		AllowHostIPC:             false,
		SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
		RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAsRange},
		SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
		FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs},
		ReadOnlyRootFilesystem:   false,
		AllowedUnsafeSysctls:     nil,
		ForbiddenSysctls:         nil,
		SeccompProfiles:          nil,
		Groups:                   []string{"system:cluster-admins", "system:nodes"},
	}
	return scc
}

func TestEnsureScc(t *testing.T) {

	var priority int32 = 10
	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		expectedScc     *securityv1.SecurityContextConstraints
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedExist:   true,
			expectedScc:     makeScc(),
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				&securityv1.SecurityContextConstraints{
					ObjectMeta:               metav1.ObjectMeta{Name: sccName, ResourceVersion: "1"},
					Priority:                 &priority, // undesired value
					AllowPrivilegedContainer: true,
					DefaultAddCapabilities:   nil,
					RequiredDropCapabilities: []corev1.Capability{"MKNOD"},
					AllowedCapabilities:      nil,
					AllowHostDirVolumePlugin: true,
					Volumes:                  []securityv1.FSType{securityv1.FSTypeHostPath, securityv1.FSTypeSecret, securityv1.FSTypeConfigMap},
					AllowHostNetwork:         false,
					AllowHostPorts:           false,
					AllowHostPID:             false,
					AllowHostIPC:             true, // not expected
					SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
					RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyRunAsAny},
					SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
					FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyRunAsAny},
					ReadOnlyRootFilesystem:   false,
					AllowedUnsafeSysctls:     []string{"dummy-value"}, // undesired value
					ForbiddenSysctls:         []string{"dummy-value"}, // undesired value
					SeccompProfiles:          []string{"dummy-value"}, // undesired value
					Groups:                   []string{"system:cluster-admins", "system:nodes"},
				},
			},
			expectedScc: makeScc(),
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
			nodeObs := &operatorv1alpha2.NodeObservability{}
			scc, err := r.ensureSecurityContextConstraints(context.TODO(), nodeObs)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}
			if tc.errExpected {
				t.Fatalf("Error expected but wasn't received")
			}

			if diff := cmp.Diff(scc, tc.expectedScc, cmpopts.IgnoreFields(securityv1.SecurityContextConstraints{}, "TypeMeta", "ObjectMeta")); diff != "" {
				t.Fatalf("unexpected diff \n%s", diff)
			}
		})
	}
}

func TestDeleteSCC(t *testing.T) {
	testCasesSCC := []struct {
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
				makeScc(),
			},
			expectedExist: false,
			errExpected:   false,
		},
	}

	for _, tc := range testCasesSCC {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client: cl,
				Scheme: test.Scheme,
				Log:    zap.New(zap.UseDevMode(true)),
			}
			nodeObs := testNodeObservability()
			err := r.deleteSecurityContextConstraints(nodeObs)
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
				Name:      sccName,
			}
			err = cl.Get(context.TODO(), name, &securityv1.SecurityContextConstraints{})
			gotExist := true
			if errors.IsNotFound(err) {
				gotExist = false
			} else if !tc.errExpected {
				t.Fatalf("unexpected error received: %v", err)
			}
			if gotExist != tc.expectedExist {
				t.Errorf("expected SCC's exist to be %t, got %t", tc.expectedExist, gotExist)
			}
		})
	}
}
