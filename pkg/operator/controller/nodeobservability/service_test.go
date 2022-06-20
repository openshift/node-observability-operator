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

package nodeobservabilitycontroller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func testControllerService(name, namespace string, selector, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       kubeRBACProxyPort,
					TargetPort: intstr.FromInt(kubeRBACProxyPort),
				},
			},
			Selector: selector,
		},
	}
}

func TestEnsureService(t *testing.T) {
	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		deployment      *appsv1.Deployment
		expectedService *corev1.Service
	}{
		{
			name: "new service",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
					},
				},
			},
			expectedService: testControllerService(
				podName,
				test.TestNamespace,
				map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
				map[string]string{injectCertsKey: podName},
			),
		},
		{
			name: "existing service, selector modified",
			existingObjects: []runtime.Object{
				testControllerService(
					podName,
					test.TestNamespace,
					map[string]string{"app": "nodeobservability-old", "nodeobs_cr": "test"},
					map[string]string{injectCertsKey: podName},
				),
			},
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
					},
				},
			},
			expectedService: testControllerService(
				podName,
				test.TestNamespace,
				map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
				map[string]string{injectCertsKey: podName},
			),
		},
		{
			name: "existing service, ports modified",
			existingObjects: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "node-observability-agent", Namespace: "test-namespace"},
					Spec: corev1.ServiceSpec{
						ClusterIP: corev1.ClusterIPNone,
						Type:      corev1.ServiceTypeClusterIP,
						Ports: []corev1.ServicePort{
							{
								Protocol:   corev1.ProtocolTCP,
								Port:       9440,
								TargetPort: intstr.FromInt(9440),
							},
						},
						Selector: map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
					},
				},
			},
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
					},
				},
			},
			expectedService: testControllerService(
				podName,
				test.TestNamespace,
				map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
				map[string]string{injectCertsKey: podName},
			),
		},
		{
			name: "existing service, service type modified",
			existingObjects: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "node-observability-agent", Namespace: "test-namespace"},
					Spec: corev1.ServiceSpec{
						ClusterIP: corev1.ClusterIPNone,
						Type:      corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								Protocol:   corev1.ProtocolTCP,
								Port:       kubeRBACProxyPort,
								TargetPort: intstr.FromInt(kubeRBACProxyPort),
							},
						},
						Selector: map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
					},
				},
			},
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "controller"},
					},
				},
			},
			expectedService: testControllerService(
				podName,
				test.TestNamespace,
				map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
				map[string]string{injectCertsKey: podName},
			),
		},
		{
			name: "existing service, extra annotations present are allowed during updates",
			existingObjects: []runtime.Object{
				testControllerService(
					podName,
					test.TestNamespace,
					map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
					map[string]string{
						"extra-annotation-key": "extra-annotation-value",
					},
				),
			},
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "controller"},
					},
				},
			},
			expectedService: testControllerService(
				podName,
				test.TestNamespace,
				map[string]string{"app": "nodeobservability", "nodeobs_cr": "test"},
				map[string]string{
					injectCertsKey:         podName,
					"extra-annotation-key": "extra-annotation-value",
				},
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client:    cl,
				Scheme:    test.Scheme,
				Namespace: test.TestNamespace,
				Log:       zap.New(zap.UseDevMode(true)),
			}
			nodeObs := &operatorv1alpha2.NodeObservability{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}

			_, err := r.ensureService(context.TODO(), nodeObs, test.TestNamespace)
			if err != nil {
				t.Fatalf("unexpected error received: %v", err)
			}
			var s corev1.Service
			err = r.Client.Get(context.Background(), types.NamespacedName{Name: tc.expectedService.Name, Namespace: tc.expectedService.Namespace}, &s)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if diff := cmp.Diff(tc.expectedService.Annotations, s.Annotations); diff != "" {
				t.Errorf("unexpected annotations\n%s", diff)
			}

			if !equality.Semantic.DeepEqual(s.Spec, tc.expectedService.Spec) {
				t.Errorf("service has unexpected configuration:\n%s", cmp.Diff(s.Spec, tc.expectedService.Spec))
			}
		})
	}
}
