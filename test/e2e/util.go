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

func waitForOperatorDeploymentStatusCondition(t *testing.T, cl client.Client, conditions ...appsv1.DeploymentCondition) error {
	t.Helper()
	return wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		dep := &appsv1.Deployment{}
		depNamespacedName := types.NamespacedName{
			Name:      "node-observability-operator-controller-manager",
			Namespace: "node-observability-operator",
		}
		if err := cl.Get(context.TODO(), depNamespacedName, dep); err != nil {
			t.Logf("failed to get deployment %s: %v", depNamespacedName.Name, err)
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
