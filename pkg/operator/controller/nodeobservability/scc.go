package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
)

const (
	sccName = "node-observability-scc"
)

// ensureSecurityContextConstraints ensures that the securitycontextconstraints exists
// Returns a Boolean value indicatiing whether it exists, a pointer to the
// securitycontextconstraints and an error when relevant
func (r *NodeObservabilityReconciler) ensureSecurityContextConstraints(ctx context.Context, nodeObs *v1alpha2.NodeObservability) (*securityv1.SecurityContextConstraints, error) {
	desired := r.desiredSecurityContextConstraints(nodeObs)
	current, err := r.currentSecurityContextConstraints(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get securitycontextconstraints %q due to: %w", sccName, err)
	} else if err != nil && errors.IsNotFound(err) {

		// creating scc since it is not found
		if err := r.createSecurityContextConstraints(ctx, desired); err != nil {
			return nil, fmt.Errorf("failed to create securitycontextconstraints %q: %w", sccName, err)
		}
		r.Log.Info("created securitycontextconstraints", "scc.Name", sccName)
		return r.currentSecurityContextConstraints(ctx)
	}

	updated, err := r.updateSecurityContextConstraintes(ctx, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update securitycontextconstraints %q due to: %w", sccName, err)
	}

	if updated {
		r.Log.V(1).Info("updated securitycontextconstraints", "scc.Name", sccName)
		return r.currentSecurityContextConstraints(ctx)
	}

	return current, nil
}

// currentSecurityContextConstraints checks that the securitycontextconstraints exists
func (r *NodeObservabilityReconciler) currentSecurityContextConstraints(ctx context.Context) (*securityv1.SecurityContextConstraints, error) {
	nameSpace := types.NamespacedName{Name: sccName}
	scc := &securityv1.SecurityContextConstraints{}
	if err := r.Client.Get(ctx, nameSpace, scc); err != nil {
		return nil, err
	}
	return scc, nil
}

// createSecurityContextConstraints creates the securitycontextconstraints
func (r *NodeObservabilityReconciler) createSecurityContextConstraints(ctx context.Context, scc *securityv1.SecurityContextConstraints) error {
	return r.Client.Create(ctx, scc)
}

// desiredSecurityContextConstraints en the desired securitycontextconstraints
func (r *NodeObservabilityReconciler) desiredSecurityContextConstraints(nodeObs *v1alpha2.NodeObservability) *securityv1.SecurityContextConstraints {

	var priority int32 = 10

	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta:               metav1.ObjectMeta{Name: sccName},
		Priority:                 &priority,
		AllowPrivilegedContainer: true,
		DefaultAddCapabilities:   nil,
		RequiredDropCapabilities: []corev1.Capability{"MKNOD"},
		AllowedCapabilities:      nil,
		AllowHostDirVolumePlugin: true,
		Volumes:                  []securityv1.FSType{securityv1.FSTypeHostPath, securityv1.FSTypeSecret, securityv1.FSTypeConfigMap},
		AllowHostNetwork:         true,
		AllowHostPorts:           false,
		AllowHostPID:             false,
		AllowHostIPC:             false,
		SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
		RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyRunAsAny},
		SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
		FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyRunAsAny},
		ReadOnlyRootFilesystem:   false,
		Groups:                   []string{"system:cluster-admins", "system:nodes"},
	}
	return scc
}

func (r *NodeObservabilityReconciler) updateSecurityContextConstraintes(ctx context.Context, current, desired *securityv1.SecurityContextConstraints) (bool, error) {
	opts := cmpopts.IgnoreFields(securityv1.SecurityContextConstraints{}, "TypeMeta", "Users", "SeccompProfiles", "AllowedUnsafeSysctls", "ForbiddenSysctls")
	if !cmp.Equal(current, desired, opts) {
		// copy over current object metadata to updated
		updated := desired.DeepCopy()
		updated.ObjectMeta = current.ObjectMeta
		if err := r.Client.Update(ctx, updated); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *NodeObservabilityReconciler) deleteSecurityContextConstraints(nodeObs *v1alpha2.NodeObservability) error {
	scc := &securityv1.SecurityContextConstraints{}
	scc.Name = sccName
	if err := r.Client.Delete(context.TODO(), scc); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}
