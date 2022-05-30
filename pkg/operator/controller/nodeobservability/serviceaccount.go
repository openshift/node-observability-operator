package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	serviceAccountName = "node-observability-sa"
)

// ensureServiceAccount ensures that the serviceaccount exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// serviceaccount and an error when relevant
func (r *NodeObservabilityReconciler) ensureServiceAccount(ctx context.Context, nodeObs *v1alpha1.NodeObservability, ns string) (bool, *corev1.ServiceAccount, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: serviceAccountName}
	desired := r.desiredServiceAccount(nodeObs, ns)
	exist, current, err := r.currentServiceAccount(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get ServiceAccount: %v", err)
	}
	if !exist {
		if err := r.createServiceAccount(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentServiceAccount(ctx, nameSpace)
	}
	return true, current, err
}

// currentServiceAccount checks that the serviceaccount exists
func (r *NodeObservabilityReconciler) currentServiceAccount(ctx context.Context, nameSpace types.NamespacedName) (bool, *corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	if err := r.Get(ctx, nameSpace, sa); err != nil || r.Err.Set[saObj] {
		if errors.IsNotFound(err) || r.Err.NotFound[saObj] {
			return false, nil, nil
		}
		if r.Err.Set[saObj] {
			err = fmt.Errorf("failed to get ServiceAccount: simulated error")
		}
		return false, nil, err
	}
	return true, sa, nil
}

// createServiceAccount creates the serviceaccount
func (r *NodeObservabilityReconciler) createServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error {
	if err := r.Create(ctx, sa); err != nil {
		return fmt.Errorf("failed to create ServiceAccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	r.Log.Info("created ServiceAccount", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
	return nil
}

// desiredServiceAccount returns a serviceaccount object
func (r *NodeObservabilityReconciler) desiredServiceAccount(nodeObs *v1alpha1.NodeObservability, ns string) *corev1.ServiceAccount {

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      serviceAccountName,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       nodeObs.Name,
					Kind:       nodeObs.Kind,
					UID:        nodeObs.UID,
					APIVersion: nodeObs.APIVersion,
				},
			},
		},
	}
	return sa
}
