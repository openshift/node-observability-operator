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

	securityv1 "github.com/openshift/api/security/v1"
	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestEnsureScc(t *testing.T) {
	var priority int32 = 10
	nodeObs := &operatorv1alpha1.NodeObservability{}
	makeScc := func() *securityv1.SecurityContextConstraints {
		scc := securityv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sccName,
				Namespace: nodeObs.Namespace,
			},
			AllowPrivilegedContainer: true,
			AllowHostIPC:             false,
			AllowHostNetwork:         true,
			AllowHostPID:             false,
			AllowHostPorts:           false,
			// This allows us to mount the hosts /var/run/crio/crio.sock into the container
			AllowHostDirVolumePlugin: true,
			AllowedCapabilities:      nil,
			DefaultAddCapabilities:   nil,
			FSGroup: securityv1.FSGroupStrategyOptions{
				Type: securityv1.FSGroupStrategyRunAsAny,
			},
			Groups:                   []string{"system:cluster-admins", "system:nodes"},
			Priority:                 &priority,
			ReadOnlyRootFilesystem:   false,
			RequiredDropCapabilities: []corev1.Capability{"MKNOD"},
			RunAsUser: securityv1.RunAsUserStrategyOptions{
				Type: securityv1.RunAsUserStrategyRunAsAny,
			},
			SELinuxContext: securityv1.SELinuxContextStrategyOptions{
				Type: securityv1.SELinuxStrategyMustRunAs,
			},
			SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
				Type: securityv1.SupplementalGroupsStrategyRunAsAny,
			},
			Volumes: []securityv1.FSType{securityv1.FSTypeHostPath, securityv1.FSTypeSecret},
		}
		return &scc
	}
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
				makeScc(),
			},
			expectedExist: true,
			expectedScc:   makeScc(),
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
			nodeObs := &operatorv1alpha1.NodeObservability{}
			gotExist, _, err := r.ensureSecurityContextConstraints(context.TODO(), nodeObs)
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
