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
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/utils/test"
)

const (
	nodeObsInstanceName            = "nodeobservability-sample"
	kubeletCAConfigMapName         = "kubelet-serving-ca"
	srcKubeletCAConfigMapNameSpace = "openshift-config-managed"
)

func TestEnsureDaemonset(t *testing.T) {
	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		serviceaccount  *corev1.ServiceAccount
		expectedDS      *appsv1.DaemonSet
	}{
		{
			name: "New daemonset",
			existingObjects: []runtime.Object{
				makeKubeletCACM(),
			},
			expectedDS: testDaemonset(daemonSetName, test.TestNamespace, serviceAccountName).
				withTemplateAnnotation("nodeobservability.olm.openshift.io/kubelet-ca-configmap-hash", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").
				withNodeSelector(map[string]string{"node-role.kubernetes.io/worker": ""}).
				withControllerReference(nodeObsInstanceName).
				withContainers(
					testContainer(podName, "node-observability-agent:latest").
						withFieldEnv("NODE_IP", "status.hostIP").
						withCommand("node-observability-agent").
						withArgs(
							"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token",
							"--storage=/run/node-observability",
							fmt.Sprintf("--caCertFile=%s%s", kbltCAMountPath, kbltCAMountedFile),
						).
						withPrivileged().
						withVolumeMount(socketName, socketMountPath, false).
						withVolumeMount(kbltCAName, kbltCAMountPath, true).
						withVolumeMount("profiledata", "/run/node-observability", false).
						build(),
					testContainer("kube-rbac-proxy", "gcr.io/kubebuilder/kube-rbac-proxy:v0.11.0").
						withArgs(
							"--secure-listen-address=0.0.0.0:8443",
							"--upstream=http://127.0.0.1:9000/",
							fmt.Sprintf("--tls-cert-file=%s/tls.crt", certsMountPath),
							fmt.Sprintf("--tls-private-key-file=%s/tls.key", certsMountPath),
							"--logtostderr=true",
							"--v=2",
						).
						withVolumeMount(certsName, certsMountPath, true).
						build(),
				).
				withHostPathVolume(socketName, socketPath, corev1.HostPathSocket).
				withConfigMapVolume(kbltCAName, kubeletCAConfigMapName).
				withSecretVolume(certsName, "node-observability-agent").
				withEmptyDirVolume("profiledata").
				build(),
		},
		{
			name: "Update existing daemonset",
			existingObjects: []runtime.Object{
				makeKubeletCACM(),
				testDaemonset(daemonSetName, test.TestNamespace, serviceAccountName).
					withNodeSelector(map[string]string{"node-role.kubernetes.io/worker": ""}).
					withControllerReference(nodeObsInstanceName).
					withResourceVersion("1").
					withContainers(
						testContainer(podName, "node-observability-agent:latest").
							withFieldEnv("NODE_IP", "status.hostIP").
							withCommand("node-observability").
							withArgs(
								"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token",
								"--storage=/run/node-observability",
								fmt.Sprintf("--caCertFile=%s%s", kbltCAMountPath, kbltCAMountedFile),
							).
							withPrivileged().
							withVolumeMount(socketName, socketMountPath, true).
							withVolumeMount(kbltCAName, kbltCAMountPath, true).
							build(),
					).
					withHostPathVolume(socketName, socketPath, corev1.HostPathSocket).
					withConfigMapVolume(kbltCAName, kubeletCAConfigMapName).
					withSecretVolume(certsName, "node-observability-agent").
					build(),
			},
			expectedDS: testDaemonset(daemonSetName, test.TestNamespace, serviceAccountName).
				withTemplateAnnotation("nodeobservability.olm.openshift.io/kubelet-ca-configmap-hash", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").
				withNodeSelector(map[string]string{"node-role.kubernetes.io/worker": ""}).
				withControllerReference(nodeObsInstanceName).
				withResourceVersion("2").
				withContainers(
					testContainer(podName, "node-observability-agent:latest").
						withFieldEnv("NODE_IP", "status.hostIP").
						withCommand("node-observability-agent").
						withArgs(
							"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token",
							"--storage=/run/node-observability",
							fmt.Sprintf("--caCertFile=%s%s", kbltCAMountPath, kbltCAMountedFile),
						).
						withPrivileged().
						withVolumeMount(socketName, socketMountPath, false).
						withVolumeMount(kbltCAName, kbltCAMountPath, true).
						withVolumeMount("profiledata", "/run/node-observability", false).
						build(),
					testContainer("kube-rbac-proxy", "gcr.io/kubebuilder/kube-rbac-proxy:v0.11.0").
						withArgs(
							"--secure-listen-address=0.0.0.0:8443",
							"--upstream=http://127.0.0.1:9000/",
							fmt.Sprintf("--tls-cert-file=%s/tls.crt", certsMountPath),
							fmt.Sprintf("--tls-private-key-file=%s/tls.key", certsMountPath),
							"--logtostderr=true",
							"--v=2",
						).
						withVolumeMount(certsName, certsMountPath, true).
						build(),
				).
				withHostPathVolume(socketName, socketPath, corev1.HostPathSocket).
				withConfigMapVolume(kbltCAName, kubeletCAConfigMapName).
				withSecretVolume(certsName, "node-observability-agent").
				withEmptyDirVolume("profiledata").
				build(),
		},
		{
			name: "Does not exist but target CA configmap is there",
			existingObjects: []runtime.Object{
				makeKubeletCACM(),
				makeTargetKubeletCACM(),
			},
			expectedDS: testDaemonset(daemonSetName, test.TestNamespace, serviceAccountName).
				withTemplateAnnotation("nodeobservability.olm.openshift.io/kubelet-ca-configmap-hash", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").
				withNodeSelector(map[string]string{"node-role.kubernetes.io/worker": ""}).
				withControllerReference(nodeObsInstanceName).
				withResourceVersion("1").
				withContainers(
					testContainer(podName, "node-observability-agent:latest").
						withFieldEnv("NODE_IP", "status.hostIP").
						withCommand("node-observability-agent").
						withArgs(
							"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token",
							"--storage=/run/node-observability",
							fmt.Sprintf("--caCertFile=%s%s", kbltCAMountPath, kbltCAMountedFile),
						).
						withPrivileged().
						withVolumeMount(socketName, socketMountPath, false).
						withVolumeMount(kbltCAName, kbltCAMountPath, true).
						withVolumeMount("profiledata", "/run/node-observability", false).
						build(),
					testContainer("kube-rbac-proxy", "gcr.io/kubebuilder/kube-rbac-proxy:v0.11.0").
						withArgs(
							"--secure-listen-address=0.0.0.0:8443",
							"--upstream=http://127.0.0.1:9000/",
							fmt.Sprintf("--tls-cert-file=%s/tls.crt", certsMountPath),
							fmt.Sprintf("--tls-private-key-file=%s/tls.key", certsMountPath),
							"--logtostderr=true",
							"--v=2",
						).
						withVolumeMount(certsName, certsMountPath, true).
						build(),
				).
				withHostPathVolume(socketName, socketPath, corev1.HostPathSocket).
				withConfigMapVolume(kbltCAName, kubeletCAConfigMapName).
				withSecretVolume(certsName, "node-observability-agent").
				withEmptyDirVolume("profiledata").
				build(),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithRuntimeObjects(tc.existingObjects...).Build()
			r := &NodeObservabilityReconciler{
				Client:     cl,
				Scheme:     test.Scheme,
				Namespace:  test.TestNamespace,
				Log:        zap.New(zap.UseDevMode(true)),
				AgentImage: "node-observability-agent:latest",
			}
			nodeObs := &operatorv1alpha2.NodeObservability{
				ObjectMeta: metav1.ObjectMeta{Name: nodeObsInstanceName},
				Spec: operatorv1alpha2.NodeObservabilitySpec{
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			}
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: test.OperatorNamespace,
					Name:      serviceAccountName,
				},
			}
			tempCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: test.OperatorNamespace,
					Name:      kubeletCAConfigMapName,
				},
			}
			_, err := r.ensureDaemonSet(context.TODO(), nodeObs, sa, r.Namespace, tempCM)
			if err != nil {
				t.Fatalf("unexpected error received: %v", err)
			}

			ds := &appsv1.DaemonSet{}
			err = r.Client.Get(context.Background(), types.NamespacedName{Namespace: test.TestNamespace, Name: daemonSetName}, ds)
			if err != nil {
				t.Fatalf("failed to get daemonset: %v", err)
			}
			if diff := cmp.Diff(ds, tc.expectedDS); diff != "" {
				t.Errorf("resource mismatch:\n%s", diff)
			}
		})
	}
}

func TestUpdateDaemonSet(t *testing.T) {

	for _, tc := range []struct {
		name              string
		existingDaemonset *appsv1.DaemonSet
		desiredDaemonset  *appsv1.DaemonSet
		expectedDaemonset *appsv1.DaemonSet
		expectUpdate      bool
	}{
		{
			name: "image changed",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").
					build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					build(),
				).build(),
			expectUpdate: true,
		},
		{
			name: "container args changed",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").
					withArgs("--arg1=1").
					build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withArgs("--arg=value").
					build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withArgs("--arg=value").
					build(),
				).build(),
			expectUpdate: true,
		},
		{
			name: "container injected into daemonset",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
					testContainer("random-container", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
					testContainer("random-container", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			expectUpdate: false,
		},
		{
			name: "new container added to desired daemonset",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
					testContainer("agent2", "agent:v2").
						withArgs("--arg2=2").
						build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
					testContainer("agent2", "agent:v2").
						withArgs("--arg2=2").
						build(),
				).build(),
			expectUpdate: true,
		},
		{
			name: "container env modified",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").
					withEnv("env2", "ENV1").
					withEnv("env1", "Modified").
					build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withEnv("env1", "ENV1").
					build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withEnv("env1", "ENV1").
					build(),
				).build(),
			expectUpdate: true,
		},
		{
			name: "container volume mount modified and new volume mount injected",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").
					withVolumeMount("vol", "/root", false).
					withVolumeMount("random-volume", "", false).
					build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withVolumeMount("vol", "/tmp", false).
					build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withVolumeMount("vol", "/tmp", false).
					withVolumeMount("random-volume", "", false).
					build(),
				).build(),
			expectUpdate: true,
		},
		{
			name: "daemonset volumes modified and new volume injected",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").build()).
				withSecretVolume("volume", "secret").
				withSecretVolume("random-volume", "random-secret").
				build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").build()).
				withSecretVolume("volume", "secret").
				build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").build()).
				withSecretVolume("volume", "secret").
				withSecretVolume("random-volume", "random-secret").
				build(),
			expectUpdate: false,
		},
		{
			name: "security context is modified",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v1").
					withUnprivileged().
					build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withPrivileged().
					build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(testContainer("agent", "agent:v2").
					withPrivileged().
					build(),
				).build(),
			expectUpdate: true,
		},
		{
			name: "daemonset is the same",
			existingDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			desiredDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			expectedDaemonset: testDaemonset("daemonset", "test-namespace", "test-sa").
				withContainers(
					testContainer("agent", "agent:v1").
						withArgs("--arg1=1").
						build(),
				).build(),
			expectUpdate: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithObjects(tc.existingDaemonset).Build()
			r := &NodeObservabilityReconciler{
				Client:     cl,
				Scheme:     test.Scheme,
				Namespace:  test.TestNamespace,
				Log:        zap.New(zap.UseDevMode(true)),
				AgentImage: "node-observability-agent:latest",
			}
			updated, err := r.updateDaemonset(context.Background(), tc.existingDaemonset, tc.desiredDaemonset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectUpdate != updated {
				t.Errorf("expected update to be %t, instead was %t", tc.expectUpdate, updated)
			}
			currentDaemonset := &appsv1.DaemonSet{}
			err = r.Client.Get(context.Background(),
				types.NamespacedName{
					Namespace: tc.expectedDaemonset.Namespace,
					Name:      tc.expectedDaemonset.Name},
				currentDaemonset)
			if err != nil {
				t.Fatalf("failed to get existing daemonset: %v", err)
			}

			if diff := cmp.Diff(currentDaemonset.Spec, tc.expectedDaemonset.Spec); diff != "" {
				t.Fatalf("daemonset spec mismatch:\n%s", diff)
			}
		})
	}
}

func TestHasSecurityContextChanged(t *testing.T) {
	for _, tc := range []struct {
		name      string
		currentSC *corev1.SecurityContext
		desiredSC *corev1.SecurityContext
		changed   bool
	}{
		{
			name:      "current Privileged is nil",
			currentSC: &corev1.SecurityContext{},
			desiredSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(false)},
			changed:   true,
		},
		{
			// should be ignored to handle defaulting
			name:      "desired Privileged is nil",
			desiredSC: &corev1.SecurityContext{},
			currentSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(false)},
			changed:   false,
		},
		{
			name:      "Privileged changes true->false",
			currentSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			desiredSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(false)},
			changed:   true,
		},
		{
			name:      "Privileged changes false->true",
			currentSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			desiredSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(false)},
			changed:   true,
		},
		{
			name:      "Privileged is same",
			currentSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			desiredSC: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			changed:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := hasSecurityContextChanged(tc.currentSC, tc.desiredSC)
			if changed != tc.changed {
				t.Errorf("expected %v, instead was %v", tc.changed, changed)
			}
		})
	}
}

func TestBuildMapHash(t *testing.T) {
	testCases := []struct {
		name         string
		inputData    map[string]string
		expectedHash string
		errExpected  bool
	}{
		{
			name: "correct hash",
			inputData: map[string]string{
				"ca-bundle.crt": "cadata",
			},
			expectedHash: "f5d3280e9b466526a60b8de15465941e612fbbde5481e638e913925cc4dc99e9",
			errExpected:  false,
		},
		{
			name:         "empty data",
			inputData:    map[string]string{},
			expectedHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			errExpected:  false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotHash, err := buildMapHash(tc.inputData)
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("unexpected error received: %v", err)
				}
				return
			}
			if tc.errExpected {
				t.Fatalf("error expected but wasn't received")
			}
			if gotHash != tc.expectedHash {
				t.Errorf("unexpected hash: %s", gotHash)
			}
		})
	}
}

type testDaemonsetBuilder struct {
	name                string
	namespace           string
	serviceAccount      string
	version             string
	containers          []corev1.Container
	ownerReference      []metav1.OwnerReference
	volumes             []corev1.Volume
	nodeSelector        map[string]string
	templateAnnotations map[string]string
}

func testDaemonset(name, namespace, serviceAccount string) *testDaemonsetBuilder {
	return &testDaemonsetBuilder{
		name:           name,
		namespace:      namespace,
		serviceAccount: serviceAccount,
	}
}

func (b *testDaemonsetBuilder) withNodeSelector(selector map[string]string) *testDaemonsetBuilder {
	b.nodeSelector = selector
	return b
}

func (b *testDaemonsetBuilder) withResourceVersion(version string) *testDaemonsetBuilder {
	b.version = version
	return b
}

func (b *testDaemonsetBuilder) withContainers(containers ...corev1.Container) *testDaemonsetBuilder {
	b.containers = containers
	return b
}

func (b *testDaemonsetBuilder) withControllerReference(name string) *testDaemonsetBuilder {
	b.ownerReference = []metav1.OwnerReference{
		{
			APIVersion:         operatorv1alpha2.GroupVersion.Identifier(),
			Kind:               "NodeObservability",
			Name:               name,
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
		},
	}
	return b
}

func (b *testDaemonsetBuilder) withHostPathVolume(name, path string, ptype corev1.HostPathType) *testDaemonsetBuilder {
	b.volumes = append(b.volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
				Type: &ptype,
			},
		},
	})
	return b
}

func (b *testDaemonsetBuilder) withConfigMapVolume(name, cmname string) *testDaemonsetBuilder {
	b.volumes = append(b.volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmname,
				},
			},
		},
	})
	return b
}

func (b *testDaemonsetBuilder) withSecretVolume(name, sname string) *testDaemonsetBuilder {
	b.volumes = append(b.volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: sname,
			},
		},
	})
	return b
}

func (b *testDaemonsetBuilder) withTemplateAnnotation(key, value string) *testDaemonsetBuilder {
	if b.templateAnnotations == nil {
		b.templateAnnotations = map[string]string{}
	}
	b.templateAnnotations[key] = value
	return b
}

func (b *testDaemonsetBuilder) withEmptyDirVolume(name string) *testDaemonsetBuilder {
	b.volumes = append(b.volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	return b
}

func (b *testDaemonsetBuilder) build() *appsv1.DaemonSet {
	labels := labelsForNodeObservability(nodeObsInstanceName)
	d := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            daemonSetName,
			Namespace:       test.TestNamespace,
			OwnerReferences: b.ownerReference,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForNodeObservability(nodeObsInstanceName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: b.templateAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers:                    b.containers,
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 defaultScheduler,
					ServiceAccountName:            b.serviceAccount,
					Volumes:                       b.volumes,
					NodeSelector:                  b.nodeSelector,
					TerminationGracePeriodSeconds: pointer.Int64(45),
				},
			},
		},
	}
	if b.version != "" {
		d.ResourceVersion = b.version
	} else {
		d.ResourceVersion = "1"
	}
	return d
}

type testContainerBuilder struct {
	name            string
	image           string
	args            []string
	command         []string
	env             []corev1.EnvVar
	volumeMounts    []corev1.VolumeMount
	securityContext *corev1.SecurityContext
}

func testContainer(name, image string) *testContainerBuilder {
	return &testContainerBuilder{
		name:  name,
		image: image,
	}
}

func (b *testContainerBuilder) withEnv(name, value string) *testContainerBuilder {
	b.env = append(b.env, corev1.EnvVar{Name: name, Value: value})
	return b
}

func (b *testContainerBuilder) withFieldEnv(name, field string) *testContainerBuilder {
	b.env = append(b.env, corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: field,
			},
		},
	})
	return b
}

func (b *testContainerBuilder) withArgs(args ...string) *testContainerBuilder {
	b.args = args
	return b
}

func (b *testContainerBuilder) withCommand(command ...string) *testContainerBuilder {
	b.command = command
	return b
}

func (b *testContainerBuilder) withVolumeMount(name, path string, readOnly bool) *testContainerBuilder {
	b.volumeMounts = append(b.volumeMounts, corev1.VolumeMount{
		Name:      name,
		MountPath: path,
		ReadOnly:  readOnly,
	})
	return b
}

func (b *testContainerBuilder) withPrivileged() *testContainerBuilder {
	b.securityContext = &corev1.SecurityContext{
		Privileged: pointer.Bool(true),
	}
	return b
}

func (b *testContainerBuilder) withUnprivileged() *testContainerBuilder {
	b.securityContext = &corev1.SecurityContext{
		Privileged: pointer.Bool(false),
	}
	return b
}

func (b *testContainerBuilder) build() corev1.Container {
	return corev1.Container{
		Name:            b.name,
		Image:           b.image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         b.command,
		Args:            b.args,
		Env:             b.env,
		VolumeMounts:    b.volumeMounts,
		SecurityContext: b.securityContext,
	}
}

func makeKubeletCACM() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeletCAConfigMapName,
			Namespace: srcKubeletCAConfigMapNameSpace,
		},
		Data: map[string]string{
			"ca-bundle.crt": "empty",
		},
	}
}

func makeTargetKubeletCACM() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeletCAConfigMapName,
			Namespace: test.OperatorNamespace,
		},
		Data: map[string]string{
			"ca-bundle.crt": "empty",
		},
	}
}
