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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

var (
	ctx = context.Background()
)

var _ = Describe("Node Observability Operator end-to-end test suite", Ordered, func() {
	var (
		nodeobservability *operatorv1alpha1.NodeObservability
	)

	BeforeAll(func() {
		nodeobservability = testNodeObservability(defaultTestName)
		By("deploying Node Observability Agents", func() {
			Expect(k8sClient.Create(ctx, nodeobservability)).To(Succeed(), "test NodeObservability resource created")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				dsNamespacedName := types.NamespacedName{
					Name:      "node-observability-ds",
					Namespace: testNamespace,
				}
				Expect(client.IgnoreNotFound(k8sClient.Get(ctx, dsNamespacedName, ds))).To(Succeed())
				return ds.Status.NumberReady != 0 && ds.Status.DesiredNumberScheduled == ds.Status.NumberReady
			}, 60, time.Second).Should(BeTrue(), "number of ready agents != number of desired agents")
		})
	})
	Context("Happy Path scenario - single scrape is initiated and it is expected to succeed", func() {
		var (
			nodeobservabilityRun *operatorv1alpha1.NodeObservabilityRun
		)
		BeforeEach(func() {
			nodeobservabilityRun = testNodeObservabilityRun(defaultTestName)

		})

		It("runs Node Observability scrape", func() {

			By("by initiating the scrape", func() {
				Expect(k8sClient.Create(ctx, nodeobservabilityRun)).To(Succeed(), "test NodeObservabilityRun resource created")
			})

			By("by collecting status", func() {
				run := &operatorv1alpha1.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      defaultTestName,
					Namespace: testNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, run)).To(Succeed())
					return run.Status.FinishedTimestamp.IsZero()
				}, 360, time.Second).Should(BeFalse())
				Expect(run.Status.FailedAgents).To(BeEmpty())
			})

		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun)).To(Succeed())
		})
	})

	Context("UnHappy Path scenario - When a concurrent scrape is initiated", func() {
		const (
			run1     = "run1"
			run2     = "run2"
			testName = defaultTestName
		)
		var (
			nodeobservabilityRun1 *operatorv1alpha1.NodeObservabilityRun
			nodeobservabilityRun2 *operatorv1alpha1.NodeObservabilityRun
		)
		BeforeEach(func() {
			nodeobservabilityRun1 = testNodeObservabilityRun(run1)
			nodeobservabilityRun2 = testNodeObservabilityRun(run2)
		})

		It("runs Node Observability scrape", func() {

			By("by creating NORs run2 right after previous test", func() {
				Expect(k8sClient.Create(ctx, nodeobservabilityRun1)).To(Succeed(), "test NodeObservabilityRun resource created")
				time.Sleep(time.Second)
				Expect(k8sClient.Create(ctx, nodeobservabilityRun2)).To(Succeed(), "test NodeObservabilityRun resource created")
			})

			By("by verifying successful status for run1", func() {
				firstrun := &operatorv1alpha1.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      run1,
					Namespace: testNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, firstrun)).To(Succeed())
					return firstrun.Status.FinishedTimestamp.IsZero()
				}, 60, time.Second).Should(BeFalse())
				Expect(firstrun.Status.FailedAgents).To(BeEmpty(), firstrun.Name+" should not have failed agents")
			})

			By("by verifying failed status for run2", func() {
				secondrun := &operatorv1alpha1.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      run2,
					Namespace: testNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, secondrun)).To(Succeed())
					return secondrun.Status.FinishedTimestamp.IsZero()
				}, 60, time.Second).Should(BeFalse())
				Expect(len(secondrun.Status.FailedAgents) > 0).To(BeTrue())
			})
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun2)).To(Succeed())
		})

	})
	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, nodeobservability)).To(Succeed())
	})
})
