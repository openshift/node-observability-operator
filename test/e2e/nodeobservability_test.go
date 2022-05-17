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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

var (
	ctx = context.Background()
)

var _ = Describe("Node Observability Operator end-to-end test suite", func() {

	Context("Happy Path scenario - single scrape is initiated and it is expected to succeed", func() {
		var (
			nodeobservability    *operatorv1alpha1.NodeObservability
			nodeobservabilityRun *operatorv1alpha1.NodeObservabilityRun
		)
		BeforeEach(func() {
			nodeobservability = testNodeObservability()
			nodeobservabilityRun = testNodeObservabilityRun()
		})

		It("runs Node Observability scrape", func() {
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
				}, 60, time.Second).Should(BeTrue())
			})

			By("by initiating the scrape", func() {
				Expect(k8sClient.Create(ctx, nodeobservabilityRun)).To(Succeed(), "test NodeObservabilityRun resource created")
			})

			By("by collecting status", func() {
				run := &operatorv1alpha1.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      testName,
					Namespace: testNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, run)).To(Succeed())
					return run.Status.FinishedTimestamp.IsZero()
				}, 60, time.Second).Should(BeFalse())
				Expect(run.Status.FailedAgents).To(BeEmpty())
			})

		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, nodeobservability)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun)).To(Succeed())
		})
	})
})
