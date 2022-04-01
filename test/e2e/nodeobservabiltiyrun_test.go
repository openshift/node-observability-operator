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

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/node-observability-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	//"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

//var cfg *rest.Config
//var k8sClient client.Client
//var testEnv *envtest.Environment

const (
	nodeobservability = "nodeobservability-sample"
	image             = "quay.io/josefkarasek/node-observability-agent:latest"
)

func TestAgentAvailable(t *testing.T) {
	if err := waitForDaemonsetStatus(t, kubeClient); err != nil {
		t.Errorf("Did not get expected available condition: %v", err)
	}
}
func TestNodeObservabilityRun(t *testing.T) {
	var (
		err error
	)
	if err := v1alpha1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err = initKubeClient(); err != nil {
		fmt.Printf("Failed to create kube client: %v\n", err)
		os.Exit(1)
	}
	if err = ensureNodeObservabilityResource(); err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create NodeObservability Resource in ns %s: %v\n", operandNamespace, err)
	}
	nodeObsRun := ensureNodeObservabilityRunResource()
	if err = kubeClient.Create(context.TODO(), nodeObsRun); err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
	}
}
func ensureNodeObservabilityResource() error {
	spec := v1alpha1.NodeObservabilitySpec{
		Labels: map[string]string{},
		Image:  image,
	}

	nodeObs := v1alpha1.NodeObservability{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeobservability,
			Namespace: operandNamespace,
		},
		Spec: spec,
	}
	return kubeClient.Create(context.TODO(), &nodeObs)
}
func ensureNodeObservabilityRunResource() *v1alpha1.NodeObservabilityRun {
	spec := v1alpha1.NodeObservabilityRunSpec{
		NodeObservabilityRef: &v1alpha1.NodeObservabilityRef{
			Name: nodeobservability,
		},
	}

	nodeObs := v1alpha1.NodeObservabilityRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeobservability,
			Namespace: operandNamespace,
		},
		Spec: spec,
	}
	return &nodeObs
}
func TestIfProfilingIsFinished(t *testing.T) {
	nodeObsRun := ensureNodeObservabilityRunResource()
	err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: nodeobservability, Namespace: operandNamespace}, nodeObsRun)
	if err != nil {
		t.Fatalf("Failed to get NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
	}
	time.Sleep(30 * time.Second)
	isFinished := nodeObsRun.Status.FinishedTimestamp
	if isFinished != nil && !isFinished.IsZero() {
		fmt.Printf("Agent Profiling is Finished")
	} else {
		t.Fatalf("Agent Profiling is Failed")
	}
}
func TestIfAnyFailedAgents(t *testing.T) {
	nodeObsRun := ensureNodeObservabilityRunResource()
	err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: nodeobservability, Namespace: operandNamespace}, nodeObsRun)
	if err != nil {
		t.Fatalf("Failed to get NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
	}
	failedAgents := nodeObsRun.Status.FailedAgents
	if failedAgents != nil {
		t.Fatalf("Some agents might have failed %v", failedAgents)
	}
}

func TestRunAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}
