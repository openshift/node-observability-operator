package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	serviceAccountName = "node-observability-sa"
)

// ensureServiceAccount ensures that the serviceaccount exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// serviceaccount and an error when relevant
func (r *NodeObservabilityReconciler) ensureServiceAccount(ctx context.Context, nodeObs *v1alpha1.NodeObservability, ns string) (*corev1.ServiceAccount, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: serviceAccountName}

	desired := r.desiredServiceAccount(nodeObs, ns)
	if err := controllerutil.SetControllerReference(nodeObs, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set the controller reference for serviceaccount %q: %w", nameSpace.Name, err)
	}

	_, err := r.currentServiceAccount(ctx, nameSpace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get serviceaccount %s/%s due to: %w", nameSpace.Namespace, nameSpace.Name, err)
	} else if err != nil && errors.IsNotFound(err) {

		// creating serviceaccount since it is not found
		if err := r.createServiceAccount(ctx, desired); err != nil {
			return nil, err
		}
		r.Log.V(1).Info("successfully created serviceaccount", "sa.name", nameSpace.Name, "sa.namespace", nameSpace.Namespace)
		return r.currentServiceAccount(ctx, nameSpace)
	}

	return desired, r.Update(ctx, desired)
}

// currentServiceAccount checks that the serviceaccount exists
func (r *NodeObservabilityReconciler) currentServiceAccount(ctx context.Context, nameSpace types.NamespacedName) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	if err := r.Get(ctx, nameSpace, sa); err != nil {
		return nil, fmt.Errorf("failed to get serviceaccount %s/%s due to: %w", nameSpace.Namespace, nameSpace.Name, err)
	}
	return sa, nil
}

// createServiceAccount creates the serviceaccount
func (r *NodeObservabilityReconciler) createServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error {
	if err := r.Create(ctx, sa); err != nil {
		return fmt.Errorf("failed to create serviceaccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	r.Log.Info("created serviceaccount", "sa.Namespace", sa.Namespace, "sa.Name", sa.Name)
	return nil
}

// desiredServiceAccount returns a serviceaccount object
func (r *NodeObservabilityReconciler) desiredServiceAccount(nodeObs *v1alpha1.NodeObservability, ns string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      serviceAccountName,
		},
	}
}
