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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func TestEnsureDaemonset(t *testing.T) {
	makeDaemonset := func() *appsv1.DaemonSet {
		nodeObs := &operatorv1alpha1.NodeObservability{}
		ls := labelsForNodeObservability(daemonSetName)
		tgp := int64(30)
		vst := corev1.HostPathSocket
		privileged := true
		ds := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSetName,
				Namespace: nodeObs.Namespace,
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: ls,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: ls,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Image:           nodeObs.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            podName,
							// TODO - this will change once the shell script in the node-observability-agent is
							// finalized
							Command:                  []string{"/bin/sh", "-c", "curl --unix-socket /var/run/crio/crio.sock http://localhost/debug/pprof/profile > /mnt/crio-${NODE_IP}_$(date +\"%F-%T.%N\").out && sleep 3600"},
							Resources:                corev1.ResourceRequirements{},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Env: []corev1.EnvVar{{
								Name: "NODE_IP",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "status.hostIP",
									},
								},
							}},
							VolumeMounts: []corev1.VolumeMount{{
								MountPath: mountPath,
								Name:      socketName,
								ReadOnly:  false,
							}},
						}},
						DNSPolicy:                     corev1.DNSClusterFirst,
						RestartPolicy:                 corev1.RestartPolicyAlways,
						SchedulerName:                 defaultScheduler,
						ServiceAccountName:            serviceAccountName,
						TerminationGracePeriodSeconds: &tgp,
						Volumes: []corev1.Volume{{
							Name: socketName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: path,
									Type: &vst,
								},
							},
						}},
						NodeSelector: nodeObs.Spec.Labels,
					},
				},
			},
		}
		return &ds
	}

	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		expectedExist   bool
		expectedDS      *appsv1.DaemonSet
		errExpected     bool
	}{
		{
			name:            "Does not exist",
			existingObjects: []runtime.Object{},
			expectedExist:   true,
			expectedDS:      makeDaemonset(),
		},
		{
			name: "Exists",
			existingObjects: []runtime.Object{
				makeDaemonset(),
			},
			expectedExist: true,
			expectedDS:    makeDaemonset(),
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
			_, serviceAccount, err := r.ensureServiceAccount(context.TODO(), nodeObs)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}
			r.Log.Info(fmt.Sprintf("Service Account : %s", serviceAccount.Name))

			gotExist, _, err := r.ensureDaemonSet(context.TODO(), nodeObs, serviceAccount)
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
