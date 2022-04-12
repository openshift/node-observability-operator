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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	//"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	//+kubebuilder:scaffold:imports
	"github.com/openshift/node-observability-operator/api/v1alpha1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

//var cfg *rest.Config
//var k8sClient client.Client
//var testEnv *envtest.Environment

const (
	nodeobservability = "nodeobservability-sample"
	image             = "registry.build01.ci.openshift.org/ci-op-p5w65rq6/release:latest"
)

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
	nodeObs := ensureNodeObservabilityResource()
	if err = kubeClient.Create(context.TODO(), nodeObs); err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create NodeObservability Resource in ns %s: %v\n", operandNamespace, err)
	}
	nodeObsRun := ensureNodeObservabilityRunResource()
	if err = kubeClient.Create(context.TODO(), nodeObsRun); err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
	}
}
func ensureNodeObservabilityResource() *v1alpha1.NodeObservability {
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
	return &nodeObs
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
func TestAgentAvailable(t *testing.T) {
	if err := waitForDaemonsetStatus(t, kubeClient); err != nil {
		t.Errorf("Did not get expected available condition: %v", err)
	}
}
func TestIfNodeObservabilityRunFinished(t *testing.T) {
	if err := IfFinished(t); err != nil {
		t.Errorf("Node Observability Run has failed: %v", err)
	}
	if err := ensureIfNoAgentsFailed(); err != nil {
		t.Errorf("Node Observability Run has failed in ns %s: %v\n", operandNamespace, err)
	}
}
func TestIfNodeObservabilityDestroyed(t *testing.T) {
	nodeObsRun := ensureNodeObservabilityRunResource()
	err := kubeClient.Delete(context.TODO(), nodeObsRun)
	if err != nil {
		t.Errorf("Failed to destroy NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
	}
	nodeObs := ensureNodeObservabilityResource()
	err = kubeClient.Delete(context.TODO(), nodeObs)
	if err != nil {
		t.Errorf("Failed to destroy NodeObservability Resource in ns %s: %v\n", operandNamespace, err)
	}
}

func IfFinished(t *testing.T) error {
	t.Helper()
	return wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		nodeObsRun := ensureNodeObservabilityRunResource()
		err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: nodeobservability, Namespace: operandNamespace}, nodeObsRun)
		if err != nil {
			t.Logf("Failed to get NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
			return false, err
		}
		isFinished := nodeObsRun.Status.FinishedTimestamp
		if isFinished != nil && !isFinished.IsZero() && len(nodeObsRun.Status.FailedAgents) == 0 {
			return true, nil
		}
		return false, err

	})
}
func ensureIfNoAgentsFailed() error {
	nodeObsRun := ensureNodeObservabilityRunResource()
	err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: nodeobservability, Namespace: operandNamespace}, nodeObsRun)
	if err != nil {
		fmt.Printf("Failed to get NodeObservabilityRun Resource in ns %s: %v\n", operandNamespace, err)
		return err
	}
	failedAgents := nodeObsRun.Status.FailedAgents
	if failedAgents != nil {
		fmt.Printf("Some agents might have failed %v", failedAgents)
		return err
	}
	return nil
}
func TestRunAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}
