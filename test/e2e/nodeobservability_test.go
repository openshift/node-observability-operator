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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
)

var (
	ctx = context.Background()
)

const (
	hostnameLabel   = "kubernetes.io/hostname"
	workerRoleLabel = "node-role.kubernetes.io/worker"
)

var _ = Describe("Node Observability Operator end-to-end test suite", Ordered, func() {
	var (
		nodeobservability *operatorv1alpha2.NodeObservability
	)

	BeforeAll(func() {
		nodeobservability = testNodeObservability()
		By("deploying Node Observability Agents", func() {
			Expect(k8sClient.Create(ctx, nodeobservability)).To(Succeed(), "NodeObservability resource is expected to be created")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				dsNamespacedName := types.NamespacedName{
					Name:      "node-observability-agent",
					Namespace: operatorNamespace,
				}
				Expect(client.IgnoreNotFound(k8sClient.Get(ctx, dsNamespacedName, ds))).To(Succeed())
				return ds.Status.NumberReady != 0 && ds.Status.DesiredNumberScheduled == ds.Status.NumberReady
			}, 60, time.Second).Should(BeTrue(), "number of ready agents != number of desired agents")
		})
	})

	Context("Happy Path scenario - single scrape is initiated and it is expected to succeed", func() {
		var (
			nodeobservabilityRun *operatorv1alpha2.NodeObservabilityRun
		)
		BeforeEach(func() {
			nodeobservabilityRun = testNodeObservabilityRun(defaultTestName)
		})

		It("runs Node Observability scrape", func() {

			By("by initiating the scrape", func() {
				Expect(k8sClient.Create(ctx, nodeobservabilityRun)).To(Succeed(), "NodeObservabilityRun resource is expected to be created")
			})

			By("by collecting status", func() {
				run := &operatorv1alpha2.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      defaultTestName,
					Namespace: operatorNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, run)).To(Succeed())
					return run.Status.FinishedTimestamp.IsZero()
				}, 900, time.Second).Should(BeFalse())
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
			nodeobservabilityRun1 *operatorv1alpha2.NodeObservabilityRun
			nodeobservabilityRun2 *operatorv1alpha2.NodeObservabilityRun
		)
		BeforeEach(func() {
			nodeobservabilityRun1 = testNodeObservabilityRun(run1)
			nodeobservabilityRun2 = testNodeObservabilityRun(run2)
		})

		It("runs Node Observability scrape", func() {

			By("by creating NORs run2 right after previous test", func() {
				Expect(k8sClient.Create(ctx, nodeobservabilityRun1)).To(Succeed(), "NodeObservabilityRun resource is expected to be created")
				time.Sleep(time.Second)
				Expect(k8sClient.Create(ctx, nodeobservabilityRun2)).To(Succeed(), "NodeObservabilityRun resource is expected to be created")
			})

			By("by verifying successful status for run1", func() {
				firstrun := &operatorv1alpha2.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      run1,
					Namespace: operatorNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, firstrun)).To(Succeed())
					return firstrun.Status.FinishedTimestamp.IsZero()
				}, 600, time.Second).Should(BeFalse())
				Expect(firstrun.Status.FailedAgents).To(BeEmpty(), firstrun.Name+" should not have failed agents")
			})

			By("by verifying failed status for run2", func() {
				secondrun := &operatorv1alpha2.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      run2,
					Namespace: operatorNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, secondrun)).To(Succeed())
					return secondrun.Status.FinishedTimestamp.IsZero()
				}, 600, time.Second).Should(BeFalse())
				Expect(len(secondrun.Status.FailedAgents) > 0).To(BeTrue())
			})
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun2)).To(Succeed())
		})
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			GinkgoWriter.Println("Cleaning up NOB")
			Expect(k8sClient.Delete(ctx, nodeobservability)).To(Succeed())
			GinkgoWriter.Println("Waiting for NOBMC to disappear")
			nobmc := &operatorv1alpha2.NodeObservabilityMachineConfig{}
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, nobmc))
			}, 900, time.Second).Should(BeTrue())
		} else {
			// keep NOB CR to preserve all the dependent objects
			GinkgoWriter.Println("Node Observability test has failed, skipping the cleanup of NOB")
			// aborting the rest of the tests as they create their own NOBs
			// which will result into a conflict
			AbortSuite("Aborting whole test suite to avoid conflicts")
		}
	})
})

var _ = Describe("Node Observability Operator end-to-end test suite using v1alpha1", Ordered, func() {
	var (
		nodeobservability *operatorv1alpha1.NodeObservability
	)

	BeforeAll(func() {
		// observe only 1 node to speed up the test
		nodeobservability = &operatorv1alpha1.NodeObservability{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: operatorv1alpha1.NodeObservabilitySpec{
				Labels: map[string]string{
					hostnameLabel: nodesWithoutOperator.Items[0].Labels[hostnameLabel],
				},
				Type: operatorv1alpha1.CrioKubeletNodeObservabilityType,
			},
		}
		By("deploying Node Observability Agents", func() {
			Expect(k8sClient.Create(ctx, nodeobservability)).To(Succeed(), "NodeObservability resource is expected to be created")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				dsNamespacedName := types.NamespacedName{
					Name:      "node-observability-agent",
					Namespace: operatorNamespace,
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
			nodeobservabilityRun = &operatorv1alpha1.NodeObservabilityRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultTestName,
					Namespace: operatorNamespace,
				},
				Spec: operatorv1alpha1.NodeObservabilityRunSpec{
					NodeObservabilityRef: &operatorv1alpha1.NodeObservabilityRef{
						Name: "cluster",
					},
				},
			}
		})

		It("runs Node Observability scrape", func() {

			By("by initiating the scrape", func() {
				Expect(k8sClient.Create(ctx, nodeobservabilityRun)).To(Succeed(), "NodeObservabilityRun resource is expected to be created")
				GinkgoWriter.Println("Creation of NOB run done")
			})

			By("by collecting status", func() {
				run := &operatorv1alpha1.NodeObservabilityRun{}
				runNamespacedName := types.NamespacedName{
					Name:      defaultTestName,
					Namespace: operatorNamespace,
				}
				Eventually(func() bool {
					Expect(k8sClient.Get(ctx, runNamespacedName, run)).To(Succeed(), "NOB run is expected to exist")
					return run.Status.FinishedTimestamp.IsZero()
				}, 600, time.Second).Should(BeFalse())
				Expect(run.Status.FailedAgents).To(BeEmpty(), "Failed agent list is expected to be empty")
			})
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, nodeobservabilityRun)).To(Succeed())
		})
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			GinkgoWriter.Println("Cleaning up NOB")
			Expect(k8sClient.Delete(ctx, nodeobservability)).To(Succeed())
			GinkgoWriter.Println("Waiting for NOBMC to disappear")
			nobmc := &operatorv1alpha1.NodeObservabilityMachineConfig{}
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, nobmc))
			}, 600, time.Second).Should(BeTrue())
		} else {
			// keep NOB CR to preserve all the dependent objects
			GinkgoWriter.Println("Node Observability test for v1alpha1 has failed, skipping the cleanup of NOB")
			// aborting the rest of the tests as they create their own NOBs
			// which will result into a conflict
			AbortSuite("Aborting whole test suite to avoid conflicts")
		}
	})
})
