package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
)

const (
	apiGroup                     = "rbac.authorization.k8s.io"
	clusterRoleName              = "node-observability-cr"
	clusterRole                  = "ClusterRole"
	clusterRoleBindingName       = "node-observability-crb"
	serviceAccount               = "ServiceAccount"
	get                          = "get"
	create                       = "create"
	list                         = "list"
	nodes                        = "nodes"
	nodesProxy                   = "nodes/proxy"
	pods                         = "pods"
	urlStatus                    = "/node-observability-status"
	urlPprof                     = "/node-observability-pprof"
	secGroup                     = "security.openshift.io"
	authnGroup                   = "authentication.k8s.io"
	authzGroup                   = "authorization.k8s.io"
	secResource                  = "securitycontextconstraints"
	secResourceName              = "hostnetwork-v2"
	tokenreviewsResource         = "tokenreviews"
	subjectaccessreviewsResource = "subjectaccessreviews"
	use                          = "use"
)

// ensureClusterRole ensures that the clusterrole exists
// Returns a Boolean value indicating whether it exists, a pointer to
// cluster role and an error when relevant
func (r *NodeObservabilityReconciler) ensureClusterRole(ctx context.Context, nodeObs *v1alpha2.NodeObservability) (bool, *rbacv1.ClusterRole, error) {
	nameSpace := types.NamespacedName{Name: clusterRoleName}
	desired := r.desiredClusterRole(nodeObs)
	exist, current, err := r.currentClusterRole(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get ClusterRole: %w", err)
	}
	if !exist {
		if err := r.createClusterRole(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentClusterRole(ctx, nameSpace)
	}
	return true, current, err
}

// currentClusterRole checks that the clusterrole  exists
func (r *NodeObservabilityReconciler) currentClusterRole(ctx context.Context, nameSpace types.NamespacedName) (bool, *rbacv1.ClusterRole, error) {
	cr := &rbacv1.ClusterRole{}
	if err := r.Get(ctx, nameSpace, cr); err != nil || r.Err.Set[crObj] {
		if errors.IsNotFound(err) || r.Err.NotFound[crObj] {
			return false, nil, nil
		}
		if r.Err.Set[crObj] {
			err = fmt.Errorf("failed to get ClusterRole: simulated error")
		}
		return false, nil, err
	}
	return true, cr, nil
}

// createClusterRole creates the clusterrole
func (r *NodeObservabilityReconciler) createClusterRole(ctx context.Context, cr *rbacv1.ClusterRole) error {
	if err := r.Create(ctx, cr); err != nil {
		return fmt.Errorf("failed to create ClusterRole %s/%s: %w", cr.Namespace, cr.Name, err)
	}
	r.Log.Info("created ClusterRole", "ClusterRole.Namespace", cr.Namespace, "ClusterRole.Name", cr.Name)
	return nil
}

// desiredClusterRole returns a clusterrole object
func (r *NodeObservabilityReconciler) desiredClusterRole(nodeObs *v1alpha2.NodeObservability) *rbacv1.ClusterRole {

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: labelsForClusterRole(clusterRoleName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{secGroup},
				Resources:     []string{secResource},
				ResourceNames: []string{secResourceName},
				Verbs:         []string{use},
			},
			{
				Verbs:     []string{get, list},
				APIGroups: []string{""},
				Resources: []string{nodes, nodesProxy, pods},
			},
			{
				Verbs:           []string{get},
				NonResourceURLs: []string{urlStatus, urlPprof},
			},
			{
				APIGroups: []string{authnGroup},
				Resources: []string{tokenreviewsResource},
				Verbs:     []string{create},
			},
			{
				APIGroups: []string{authzGroup},
				Resources: []string{subjectaccessreviewsResource},
				Verbs:     []string{create},
			},
		},
	}
	return cr
}

// ensureClusterRoleBinding ensures that the clusterrolebinding exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// clusterrolebinding and an error when relevant
func (r *NodeObservabilityReconciler) ensureClusterRoleBinding(ctx context.Context, nodeObs *v1alpha2.NodeObservability, saName, ns string) (bool, *rbacv1.ClusterRoleBinding, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: clusterRoleBindingName}
	desired := r.desiredClusterRoleBinding(nodeObs, saName, ns)
	exist, current, err := r.currentClusterRoleBinding(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get ClusterRoleBinding: %v", err)
	}
	if !exist {
		if err := r.createClusterRoleBinding(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentClusterRoleBinding(ctx, nameSpace)
	}
	return true, current, err
}

// currentClusterRoleBinding checks if the clusterrolebinding exists
func (r *NodeObservabilityReconciler) currentClusterRoleBinding(ctx context.Context, nameSpace types.NamespacedName) (bool, *rbacv1.ClusterRoleBinding, error) {
	crb := &rbacv1.ClusterRoleBinding{}

	if err := r.Get(ctx, nameSpace, crb); err != nil || r.Err.Set[crbObj] {
		if errors.IsNotFound(err) || r.Err.NotFound[crbObj] {
			return false, nil, nil
		}
		if r.Err.Set[crbObj] {
			err = fmt.Errorf("failed to get ClusterRoleiBinding: simulated error")
		}
		return false, nil, err
	}
	return true, crb, nil
}

// createClusterRoleBinding creates the clusterrolebinding
func (r *NodeObservabilityReconciler) createClusterRoleBinding(ctx context.Context, crb *rbacv1.ClusterRoleBinding) error {
	if err := r.Create(ctx, crb); err != nil {
		return fmt.Errorf("failed to create ClusterRoleBinding %s/%s: %w", crb.Namespace, crb.Name, err)
	}
	r.Log.Info("created ClusterRoleBinding", "ClusterRoleBinding.Namespace", crb.Namespace, "ClusterRoleBinding.Name", crb.Name)
	return nil
}

// desiredClusterRoleBinding returns a clusterrolebinding object
func (r *NodeObservabilityReconciler) desiredClusterRoleBinding(nodeObs *v1alpha2.NodeObservability, saName, ns string) *rbacv1.ClusterRoleBinding {

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      serviceAccount,
				Name:      saName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     clusterRole,
			Name:     clusterRoleName,
			APIGroup: apiGroup,
		},
	}
	return crb
}

// labelsForclusterRole returns the labels for selecting the resources
func labelsForClusterRole(name string) map[string]string {
	return map[string]string{"scc": "node-observability-scc-role", "role": name}
}

func (r *NodeObservabilityReconciler) deleteClusterRole(nodeObs *v1alpha2.NodeObservability) error {
	cr := &rbacv1.ClusterRole{}
	cr.Name = clusterRoleName
	if err := r.Client.Delete(context.TODO(), cr); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
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
