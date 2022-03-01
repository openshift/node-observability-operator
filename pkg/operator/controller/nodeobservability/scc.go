package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/node-observability-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	sccName = "node-observability-scc"
)

// ensureSecurityContextConstraints ensures that the securitycontextconstraints exists
// Returns a Boolean value indicatiing whether it exists, a pointer to the
// securitycontextconstraints and an error when relevant
func (r *NodeObservabilityReconciler) ensureSecurityContextConstraints(ctx context.Context, nodeObs *v1alpha1.NodeObservability) (bool, *securityv1.SecurityContextConstraints, error) {
	desired := r.desiredSecurityContextConstraints(nodeObs)
	exist, current, err := r.currentSecurityContextConstraints(ctx, nodeObs)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get SecurityContextConstraints: %w", err)
	}
	if !exist {
		if err := r.createSecurityContextConstraints(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentSecurityContextConstraints(ctx, nodeObs)
	}
	// Set NodeObservability instance as the owner and controller
	ctrl.SetControllerReference(nodeObs, desired, r.Scheme)
	return true, current, nil
}

// currentSecurityContextConstraints checks that the securitycontextconstraints exists
func (r *NodeObservabilityReconciler) currentSecurityContextConstraints(ctx context.Context, nodeObs *v1alpha1.NodeObservability) (bool, *securityv1.SecurityContextConstraints, error) {
	nameSpace := types.NamespacedName{Namespace: nodeObs.Namespace, Name: sccName}
	scc := &securityv1.SecurityContextConstraints{}
	if err := r.Get(ctx, nameSpace, scc); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, scc, nil
}

// createSecurityContextConstraints creates the securitycontextconstraints
func (r *NodeObservabilityReconciler) createSecurityContextConstraints(ctx context.Context, scc *securityv1.SecurityContextConstraints) error {
	if err := r.Create(ctx, scc); err != nil {
		return fmt.Errorf("failed to create SecurityContextConstraint %s/%s: %w", scc.Namespace, scc.Name, err)
	}
	r.Log.Info("created SecurityContextConstraints", "SecurityContextConstraints.Namespace", scc.Namespace, "SecurityContextConstraints.Name", scc.Name)
	return nil
}

// desiredSecurityContextConstraints en the desired securitycontextconstraints
func (r *NodeObservabilityReconciler) desiredSecurityContextConstraints(nodeObs *v1alpha1.NodeObservability) *securityv1.SecurityContextConstraints {

	var priority int32 = 10

	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sccName,
			Namespace: nodeObs.Namespace,
		},
		AllowPrivilegedContainer: true,
		AllowHostIPC:             false,
		AllowHostNetwork:         true,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		// This allows us to mount the hosts /var/run/crio/crio.sock into the container
		AllowHostDirVolumePlugin: true,
		AllowedCapabilities:      nil,
		DefaultAddCapabilities:   nil,
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyRunAsAny,
		},
		Groups:                   []string{"system:cluster-admins", "system:nodes"},
		Priority:                 &priority,
		ReadOnlyRootFilesystem:   false,
		RequiredDropCapabilities: []corev1.Capability{"MKNOD"},
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
		Volumes: []securityv1.FSType{securityv1.FSTypeHostPath, securityv1.FSTypeSecret},
	}
	return scc
}
