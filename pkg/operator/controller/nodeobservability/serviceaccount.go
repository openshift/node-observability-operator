package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
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
		return nil, fmt.Errorf("failed to set the controller reference for serviceaccount %q: %w", nameSpace, err)
	}

	current, err := r.currentServiceAccount(ctx, nameSpace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get serviceaccount %q due to: %w", nameSpace, err)
	} else if err != nil && errors.IsNotFound(err) {

		// creating serviceaccount since it is not found
		if err := r.createServiceAccount(ctx, desired); err != nil {
			return nil, fmt.Errorf("failed to create serviceaccount %q: %w", nameSpace, err)
		}
		r.Log.V(1).Info("successfully created serviceaccount", "sa.name", nameSpace.Name, "sa.namespace", nameSpace.Namespace)
		return r.currentServiceAccount(ctx, nameSpace)
	}

	var updated *corev1.ServiceAccount
	if updated, err = r.updateServiceAccount(ctx, current, desired); err != nil {
		return nil, fmt.Errorf("failed to update serviceaccount %q due to: %w", nameSpace, err)
	}

	r.Log.V(1).Info("successfully updated serviceaccount", "sa.name", nameSpace.Name, "sa.namespace", nameSpace.Namespace)
	return updated, nil
}

// currentServiceAccount checks that the serviceaccount exists
func (r *NodeObservabilityReconciler) currentServiceAccount(ctx context.Context, nameSpace types.NamespacedName) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	if err := r.Get(ctx, nameSpace, sa); err != nil {
		return nil, err
	}
	return sa, nil
}

// createServiceAccount creates the serviceaccount
func (r *NodeObservabilityReconciler) createServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error {
	return r.Create(ctx, sa)
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

func (r *NodeObservabilityReconciler) updateServiceAccount(ctx context.Context, current, desired *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	updatedSA := current.DeepCopy()
	var updated bool

	if !cmp.Equal(current.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		updatedSA.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
		updated = true
	}

	if updated {
		return updatedSA, r.Update(ctx, updatedSA)
	}

	return updatedSA, nil
}
