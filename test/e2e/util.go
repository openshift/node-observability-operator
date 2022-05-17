package e2e

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

const (
	testName      = "test-instance"
	testNamespace = "node-observability-operator"
	image         = "registry.ci.openshift.org/ocp/4.11:node-observability-agent"
)

// testNodeObservability - minimal CR for the test
func testNodeObservability() *operatorv1alpha1.NodeObservability {
	return &operatorv1alpha1.NodeObservability{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: operatorv1alpha1.NodeObservabilitySpec{
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			Image: image,
		},
	}
}

// testNodeObservabilityRun - minimal CR for the test
func testNodeObservabilityRun() *operatorv1alpha1.NodeObservabilityRun {
	return &operatorv1alpha1.NodeObservabilityRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: operatorv1alpha1.NodeObservabilityRunSpec{
			NodeObservabilityRef: &operatorv1alpha1.NodeObservabilityRef{
				Name: testName,
			},
		},
	}
}
