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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	//+kubebuilder:scaffold:imports
	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
const (
	operandNamespace      = "node-observability-operator"
	operandRbacController = "controller-manager"
	operandNamespaceRbac  = "system"
)

// var cfg *rest.Config
var (
	kubeClient client.Client
	Scheme     = runtime.NewScheme()
)

func init() {
	if err := clientgoscheme.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := appsv1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := securityv1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := operatorv1alpha1.AddToScheme(Scheme); err != nil {
		panic(err)
	}

}
func initKubeClient() error {
	kubeConfig, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kube config: %w", err)
	}

	kubeClient, err = client.New(kubeConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}
	return nil
}
func TestOperatorAvailable(t *testing.T) {
	expected := []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
	}
	if err := waitForOperatorDeploymentStatusCondition(t, kubeClient, expected...); err != nil {
		t.Errorf("Did not get expected available condition: %v", err)
	}
}
func TestMain(m *testing.M) {
	var (
		err error
	)
	if err = initKubeClient(); err != nil {
		fmt.Printf("Failed to create kube client: %v\n", err)
		os.Exit(1)
	}
	if err = ensureOperandNamespace(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create %s namespace: %v\n", operandNamespace, err)
	}
	if err = ensureAuthProxyClientClusterRole(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create Auth Proxy Client ClusterRole in ns %s: %v\n", operandNamespace, err)
	}
	if err = ensureAuthProxyRoleBinding(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create Auth Proxy Rolebinding in ns %s: %v\n", operandNamespace, err)
	}
	if err = ensureAuthProxyRole(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create Auth Proxy Role in ns %s: %v\n", operandNamespace, err)
	}
	if err = ensureLeaderElectionRole(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create Leader Election Role in ns %s: %v\n", operandNamespace, err)
	}
	if err = ensureLeaderElectionRoleBinding(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create Leader Election Rolebinding in ns %s: %v\n", operandNamespace, err)
	}
	if err = ensureOperandClusterRoleBinding(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create clusterrolebinding : %v\n", err)
	}
	if err = ensureOperandClusterRole(); err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Failed to create clusterrole in : %v\n", err)
	}
	exitStatus := m.Run()
	os.Exit(exitStatus)
}

func ensureOperandNamespace() error {
	return kubeClient.Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operandNamespace}})
}
func ensureAuthProxyClientClusterRole() error {
	rules := []rbacv1.PolicyRule{
		{
			NonResourceURLs: []string{"/metrics"},
			Verbs:           []string{"get"},
		},
	}

	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: operandNamespace + "-metrics-reader",
		},
		Rules: rules,
	}
	return kubeClient.Create(context.TODO(), &clusterRole)
}

func ensureAuthProxyRoleBinding() error {
	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: operandNamespace + "-proxy-rolebinding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "proxy-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      operandRbacController,
				Namespace: operandNamespaceRbac,
			},
		},
	}
	return kubeClient.Create(context.TODO(), &crb)
}
func ensureAuthProxyRole() error {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"tokenreviews"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"authorization.k8s.io"},
			Resources: []string{"subjectaccessreviews"},
			Verbs:     []string{"create"},
		},
	}

	clusterrole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: operandNamespace + "-proxy-role",
		},
		Rules: rules,
	}
	return kubeClient.Create(context.TODO(), &clusterrole)
}
func ensureLeaderElectionRoleBinding() error {
	crb := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operandNamespace + "-leader-election-rolebinding",
			Namespace: operandNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "leader-election-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      operandRbacController,
				Namespace: operandNamespaceRbac,
			},
		},
	}
	return kubeClient.Create(context.TODO(), &crb)
}
func ensureLeaderElectionRole() error {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch"},
		},
	}

	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operandNamespace + "-leader-election-role",
			Namespace: operandNamespace,
		},
		Rules: rules,
	}
	return kubeClient.Create(context.TODO(), &role)
}
func ensureOperandClusterRoleBinding() error {
	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: operandNamespace + "-manager-rolebinding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "manager-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      operandRbacController,
				Namespace: operandNamespaceRbac,
			},
		},
	}
	return kubeClient.Create(context.TODO(), &crb)
}
func ensureOperandClusterRole() error {
	rules := []rbacv1.PolicyRule{
		{
			NonResourceURLs: []string{"/debug/*"},
			Verbs:           []string{"get"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"daemonsets"},
			Verbs:     []string{"create", "get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"nodes/proxy"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"create", "get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"serviceaccounts"},
			Verbs:     []string{"create", "get", "list", "watch"},
		},
		{
			APIGroups: []string{"nodeobservability.olm.openshift.io"},
			Resources: []string{"nodeobservabilities"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"nodeobservability.olm.openshift.io"},
			Resources: []string{"nodeobservabilities/finalizers"},
			Verbs:     []string{"update"},
		},
		{
			APIGroups: []string{"nodeobservability.olm.openshift.io"},
			Resources: []string{"nodeobservabilities/status"},
			Verbs:     []string{"get", "patch", "update"},
		},
		{
			APIGroups: []string{"nodeobservability.olm.openshift.io"},
			Resources: []string{"nodeobservabilityruns"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"nodeobservability.olm.openshift.io"},
			Resources: []string{"nodeobservabilitiesruns/finalizers"},
			Verbs:     []string{"update"},
		},
		{
			APIGroups: []string{"nodeobservability.olm.openshift.io"},
			Resources: []string{"nodeobservabilitiesruns/status"},
			Verbs:     []string{"get", "patch", "update"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"clusterroles"},
			Verbs:     []string{"create", "get", "list", "patch"},
		},
		{
			APIGroups: []string{"security.openshift.io"},
			Resources: []string{"securitycontextconstraints"},
			Verbs:     []string{"create", "get", "list", "use", "watch"},
		},
	}

	clusterrole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: operandNamespace + "-manager-role",
		},
		Rules: rules,
	}
	return kubeClient.Create(context.TODO(), &clusterrole)
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}
