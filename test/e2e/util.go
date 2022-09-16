package e2e

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	//+kubebuilder:scaffold:imports
)

const (
	defaultTestName = "test-instance"
	testNamespace   = "node-observability-operator"
)

// testNodeObservability - minimal CR for the test
func testNodeObservability() *operatorv1alpha2.NodeObservability {
	return &operatorv1alpha2.NodeObservability{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: testNamespace,
		},
		Spec: operatorv1alpha2.NodeObservabilitySpec{
			NodeSelector: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			Type: operatorv1alpha2.CrioKubeletNodeObservabilityType,
		},
	}
}

// testNodeObservabilityRun - minimal CR for the test
func testNodeObservabilityRun(testName string) *operatorv1alpha2.NodeObservabilityRun {
	return &operatorv1alpha2.NodeObservabilityRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: operatorv1alpha2.NodeObservabilityRunSpec{
			NodeObservabilityRef: &operatorv1alpha2.NodeObservabilityRef{
				Name: "cluster",
			},
		},
	}
}
