package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	"github.com/lmzuccarelli/node-observability-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	secretName = "node-observability-secret"
)

// ensureSecret ensures that the secret exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// secret and an error when relevant
func (r *NodeObservabilityReconciler) ensureSecret(ctx context.Context, nodeObs *v1alpha1.NodeObservability) (bool, *corev1.Secret, error) {
	nameSpace := types.NamespacedName{Namespace: nodeObs.Namespace, Name: secretName}
	desired := r.desiredSecret(nodeObs)
	exist, current, err := r.currentSecret(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get Secret: %w", err)
	}
	if !exist {
		if err := r.createSecret(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentSecret(ctx, nameSpace)
	}
	// Set NodeObservability instance as the owner and controller
	ctrl.SetControllerReference(nodeObs, desired, r.Scheme)
	return true, current, nil
}

// currentSecret checks that the serviceaccount exists
func (r *NodeObservabilityReconciler) currentSecret(ctx context.Context, nameSpace types.NamespacedName) (bool, *corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, nameSpace, secret); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, secret, nil
}

// createSecret creates the serviceaccount
func (r *NodeObservabilityReconciler) createSecret(ctx context.Context, secret *corev1.Secret) error {
	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create Secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	r.Log.Info("created Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
	return nil
}

// desiredSecret returns a Secret object
func (r *NodeObservabilityReconciler) desiredSecret(nodeObs *v1alpha1.NodeObservability) *corev1.Secret {

	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   nodeObs.Namespace,
			Name:        secretName,
			Annotations: annotationsForSecret(),
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	return s
}

// annotationsForSecret returns the annotations for the secret
func annotationsForSecret() map[string]string {
	return map[string]string{"kubernetes.io/service-account.name": "profiling"}
}
