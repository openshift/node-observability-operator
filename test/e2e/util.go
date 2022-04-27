package e2e

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

func waitForOperatorDeploymentStatusCondition(t *testing.T, cl client.Client, interval time.Duration, timeout time.Duration, conditions ...appsv1.DeploymentCondition) error {
	t.Helper()
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		dep := &appsv1.Deployment{}
		depNamespacedName := types.NamespacedName{
			Name:      "node-observability-operator-controller-manager",
			Namespace: "node-observability-operator",
		}
		if err := cl.Get(context.TODO(), depNamespacedName, dep); err != nil {
			t.Logf("waiting to get deployment %s: %v", depNamespacedName.Name, err)
			return false, nil
		}

		expected := deploymentConditionMap(conditions...)
		current := deploymentConditionMap(dep.Status.Conditions...)
		return conditionsMatchExpected(expected, current), nil
	})
}

func deploymentConditionMap(conditions ...appsv1.DeploymentCondition) map[string]string {
	conds := map[string]string{}
	for _, cond := range conditions {
		conds[string(cond.Type)] = string(cond.Status)
	}
	return conds
}

func conditionsMatchExpected(expected, actual map[string]string) bool {
	filtered := map[string]string{}
	for k := range actual {
		if _, comparable := expected[k]; comparable {
			filtered[k] = actual[k]
		}
	}
	return reflect.DeepEqual(expected, filtered)
}

func waitForDaemonsetStatus(t *testing.T, cl client.Client, interval time.Duration, timeout time.Duration) error {
	t.Helper()
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		ds := &appsv1.DaemonSet{}
		dsNamespacedName := types.NamespacedName{
			Name:      "node-observability-ds",
			Namespace: "node-observability-operator",
		}
		if err := cl.Get(context.TODO(), dsNamespacedName, ds); err != nil {
			t.Logf("waiting to get Daemonset %s: %v", dsNamespacedName.Name, err)
			return false, nil
		}
		return true, nil
	})
}
