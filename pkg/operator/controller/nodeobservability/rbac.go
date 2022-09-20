package nodeobservabilitycontroller

import (
	"context"
	"fmt"
	"reflect"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
)

const (

	// TODO: standardize resource names to keep them compatible
	clusterRoleName        = "node-observability-operator-controller-role"
	clusterRoleBindingName = "node-observability-operator-controller-role-binding"
)

func (r *NodeObservabilityReconciler) verifyClusterRole(ctx context.Context) (bool, error) {
	cr := &rbacv1.ClusterRole{}
	if err := r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, cr); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ensureClusterRoleBinding ensures that the clusterrolebinding exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// clusterrolebinding and an error when relevant
func (r *NodeObservabilityReconciler) ensureClusterRoleBinding(ctx context.Context, nodeObs *v1alpha2.NodeObservability, saName, ns string) (*rbacv1.ClusterRoleBinding, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: clusterRoleBindingName}
	desired := r.desiredClusterRoleBinding(nodeObs, saName, ns)

	if err := controllerutil.SetControllerReference(nodeObs, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set the controller reference for clusterrolebindings %s : %w", desired.Name, err)
	}

	current, err := r.currentClusterRoleBinding(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get clusterrolebinding %q due to: %w", nameSpace, err)
	} else if err != nil && errors.IsNotFound(err) {

		// create clusterrolebinding since it is not found
		if err := r.createClusterRoleBinding(ctx, desired); err != nil {
			return nil, fmt.Errorf("failed to create clusterrolebinding %q: %w", nameSpace, err)
		}

		r.Log.Info("created clusterrolebinding", "clusterrolebinding.Namespace", nameSpace.Namespace, "clusterrolebinding.Name", nameSpace.Name)
		return r.currentClusterRoleBinding(ctx)
	}

	var updated *rbacv1.ClusterRoleBinding
	if updated, err = r.updateClusterRoleBinding(ctx, current, desired); err != nil {
		return nil, fmt.Errorf("failed to update clusterrolebinding %s/%s due to: %w", nameSpace.Namespace, nameSpace.Name, err)
	}

	return updated, nil
}

// currentClusterRoleBinding checks if the clusterrolebinding exists
func (r *NodeObservabilityReconciler) currentClusterRoleBinding(ctx context.Context) (*rbacv1.ClusterRoleBinding, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleBindingName}, crb); err != nil {
		return nil, err
	}
	return crb, nil
}

// createClusterRoleBinding creates the clusterrolebinding
func (r *NodeObservabilityReconciler) createClusterRoleBinding(ctx context.Context, crb *rbacv1.ClusterRoleBinding) error {
	return r.Create(ctx, crb)
}

// desiredClusterRoleBinding returns a clusterrolebinding object
func (r *NodeObservabilityReconciler) desiredClusterRoleBinding(nodeObs *v1alpha2.NodeObservability, saName, ns string) *rbacv1.ClusterRoleBinding {

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
	}
	return crb
}

// updateClusterRoleBindings updates the current clusterrolebindings and returns a flag to denote if the update was done.
func (r *NodeObservabilityReconciler) updateClusterRoleBinding(ctx context.Context, current *rbacv1.ClusterRoleBinding, desired *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	changed := hasClusterRoleBindingChanged(current, desired)

	if !changed {
		return current, nil
	}

	updated := current.DeepCopy()
	updated.Subjects = desired.Subjects
	updated.RoleRef = desired.RoleRef

	if err := r.Client.Update(ctx, updated); err != nil {
		return nil, err
	}

	return updated, nil
}

func (r *NodeObservabilityReconciler) deleteClusterRoleBinding(nodeObs *v1alpha2.NodeObservability) error {
	crb := &rbacv1.ClusterRoleBinding{}
	crb.Name = clusterRoleBindingName
	if err := r.Client.Delete(context.TODO(), crb); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func hasClusterRoleBindingChanged(current *rbacv1.ClusterRoleBinding, desired *rbacv1.ClusterRoleBinding) bool {
	if !(reflect.DeepEqual(current.Subjects, desired.Subjects)) || !(reflect.DeepEqual(current.RoleRef, desired.RoleRef)) {
		return true
	}

	return false
}
