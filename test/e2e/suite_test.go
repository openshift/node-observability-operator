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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	nodeobservabilityv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	//+kubebuilder:scaffold:imports
)

var cfg *rest.Config
var k8sClient client.Client

// nodesWithoutOperator is a list of nodes on which the operator is not scheduled,
// restarting them is not supposed to result in the loss of the operator logs.
var nodesWithoutOperator *corev1.NodeList

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Node Observability Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	err := nodeobservabilityv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = nodeobservabilityv1alpha2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clientgoscheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = securityv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cfg, err = config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	expected := []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
	}
	err = waitForOperatorDeploymentStatusCondition(k8sClient, expected...)
	Expect(err).NotTo(HaveOccurred(), "Operator is expected to be available")

	opNodeName, err := operatorScheduledNodeName(k8sClient)
	Expect(err).NotTo(HaveOccurred(), "Listing of operator POD is expected to succeed")
	Expect(opNodeName).NotTo(BeEmpty(), "Operator POD is expected to be scheduled")

	nodesWithoutOperator = &corev1.NodeList{}
	nodes := &corev1.NodeList{}
	Expect(k8sClient.List(ctx, nodes, client.MatchingLabels(map[string]string{workerRoleLabel: ""}))).To(Succeed(), "List of nodes is expected to be retrieved")
	Expect(len(nodes.Items)).To(Not(BeZero()))
	for _, n := range nodes.Items {
		if n.Name != opNodeName {
			nodesWithoutOperator.Items = append(nodesWithoutOperator.Items, n)
		}
	}
	Expect(len(nodesWithoutOperator.Items)).To(Not(BeZero()))
})
