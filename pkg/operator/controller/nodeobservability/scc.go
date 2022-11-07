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
	sccName = "node-observability-agent"
)

// ensureSecurityContextConstraints ensures that the securitycontextconstraints exists
// Returns a Boolean value indicatiing whether it exists, a pointer to the
// securitycontextconstraints and an error when relevant.
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
		r.Log.Info("successfully created securitycontextconstraints", "scc.name", sccName)
		return r.currentSecurityContextConstraints(ctx)
	}

	updated, err := r.updateSecurityContextConstraintes(ctx, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update securitycontextconstraints %q due to: %w", sccName, err)
	}

	if updated {
		r.Log.V(1).Info("successfully updated securitycontextconstraints", "scc.name", sccName)
		return r.currentSecurityContextConstraints(ctx)
	}

	return current, nil
}

// currentSecurityContextConstraints checks that the securitycontextconstraints exists.
func (r *NodeObservabilityReconciler) currentSecurityContextConstraints(ctx context.Context) (*securityv1.SecurityContextConstraints, error) {
	nameSpace := types.NamespacedName{Name: sccName}
	scc := &securityv1.SecurityContextConstraints{}
	if err := r.Client.Get(ctx, nameSpace, scc); err != nil {
		return nil, err
	}
	return scc, nil
}

// createSecurityContextConstraints creates the securitycontextconstraints.
func (r *NodeObservabilityReconciler) createSecurityContextConstraints(ctx context.Context, scc *securityv1.SecurityContextConstraints) error {
	return r.Client.Create(ctx, scc)
}

// desiredSecurityContextConstraints returns the desired securitycontextconstraints.
func (r *NodeObservabilityReconciler) desiredSecurityContextConstraints(nodeObs *v1alpha2.NodeObservability) *securityv1.SecurityContextConstraints {
	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta:               metav1.ObjectMeta{Name: sccName},
		Priority:                 nil,
		AllowPrivilegedContainer: true,
		DefaultAddCapabilities:   nil,
		RequiredDropCapabilities: []corev1.Capability{"MKNOD"},
		AllowedCapabilities:      nil,
		AllowHostDirVolumePlugin: true,
		Volumes:                  []securityv1.FSType{securityv1.FSTypeHostPath, securityv1.FSTypeSecret, securityv1.FSTypeConfigMap, securityv1.FSTypeEmptyDir},
		AllowHostNetwork:         false,
		AllowHostPorts:           false,
		AllowHostPID:             false,
		AllowHostIPC:             false,
		SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
		RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAsRange},
		SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
		FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs},
		ReadOnlyRootFilesystem:   false,
		AllowedUnsafeSysctls:     nil,
		ForbiddenSysctls:         nil,
		SeccompProfiles:          nil,
		Groups:                   []string{"system:cluster-admins", "system:nodes"},
	}
	return scc
}

func (r *NodeObservabilityReconciler) updateSecurityContextConstraintes(ctx context.Context, current, desired *securityv1.SecurityContextConstraints) (bool, error) {
	updatedScc := current.DeepCopy()
	updated := false

	if desired.Priority != current.Priority {
		updatedScc.Priority = desired.Priority
		updated = true
	}

	if desired.AllowPrivilegedContainer != current.AllowPrivilegedContainer {
		updatedScc.AllowPrivilegedContainer = desired.AllowPrivilegedContainer
		updated = true
	}

	if !capabilitiesEqual(desired.DefaultAddCapabilities, current.DefaultAddCapabilities) {
		updatedScc.DefaultAddCapabilities = desired.DefaultAddCapabilities
		updated = true
	}

	if !capabilitiesEqual(desired.RequiredDropCapabilities, current.RequiredDropCapabilities) {
		updatedScc.RequiredDropCapabilities = desired.RequiredDropCapabilities
		updated = true
	}

	if !capabilitiesEqual(desired.AllowedCapabilities, current.AllowedCapabilities) {
		updatedScc.AllowedCapabilities = desired.AllowedCapabilities
		updated = true
	}

	if desired.AllowHostDirVolumePlugin != current.AllowHostDirVolumePlugin {
		updatedScc.AllowHostDirVolumePlugin = desired.AllowHostDirVolumePlugin
		updated = true
	}

	if !volumesEqual(desired.Volumes, current.Volumes) {
		updatedScc.Volumes = desired.Volumes
		updated = true
	}

	if desired.AllowHostNetwork != current.AllowHostNetwork {
		updatedScc.AllowHostNetwork = desired.AllowHostNetwork
		updated = true
	}

	if desired.AllowHostPorts != current.AllowHostPorts {
		updatedScc.AllowHostPorts = desired.AllowHostPorts
		updated = true
	}

	if desired.AllowHostPID != current.AllowHostPID {
		updatedScc.AllowHostPorts = desired.AllowHostPorts
		updated = true
	}

	if desired.AllowHostIPC != current.AllowHostIPC {
		updatedScc.AllowHostIPC = desired.AllowHostIPC
		updated = true
	}

	if !cmp.Equal(desired.SELinuxContext, current.SELinuxContext) {
		updatedScc.SELinuxContext = desired.SELinuxContext
		updated = true
	}

	if !cmp.Equal(desired.RunAsUser, current.RunAsUser) {
		updatedScc.RunAsUser = desired.RunAsUser
		updated = true
	}

	if !cmp.Equal(desired.SupplementalGroups, current.SupplementalGroups) {
		updatedScc.SupplementalGroups = desired.SupplementalGroups
		updated = true
	}

	if !cmp.Equal(desired.FSGroup, current.FSGroup) {
		updatedScc.FSGroup = desired.FSGroup
		updated = true
	}
	if desired.ReadOnlyRootFilesystem != current.ReadOnlyRootFilesystem {
		updatedScc.ReadOnlyRootFilesystem = desired.ReadOnlyRootFilesystem
		updated = true
	}

	if !stringSlicesEqual(desired.AllowedUnsafeSysctls, current.AllowedUnsafeSysctls) {
		updatedScc.AllowedUnsafeSysctls = desired.AllowedUnsafeSysctls
		updated = true
	}

	if !stringSlicesEqual(desired.SeccompProfiles, current.SeccompProfiles) {
		updatedScc.SeccompProfiles = desired.SeccompProfiles
		updated = true
	}

	if !stringSlicesEqual(desired.ForbiddenSysctls, current.ForbiddenSysctls) {
		updatedScc.ForbiddenSysctls = desired.ForbiddenSysctls
		updated = true
	}

	if updated {
		if err := r.Client.Update(ctx, updatedScc); err != nil {
			return false, err
		}
	}

	return updated, nil
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

// volumesEqual compares fstype slices, order doesn't matter.
func volumesEqual(desired, current []securityv1.FSType) bool {
	if len(desired) != len(current) {
		return false
	}

	sortOpt := cmpopts.SortSlices(func(a, b securityv1.FSType) bool { return string(a) < string(b) })
	return cmp.Equal(desired, current, sortOpt)
}

// capabilitiesEqual compares capability slices, order doesn't matter.
func capabilitiesEqual(desired, current []corev1.Capability) bool {
	if len(desired) != len(current) {
		return false
	}

	sortOpt := cmpopts.SortSlices(func(a, b corev1.Capability) bool { return string(a) < string(b) })
	return cmp.Equal(desired, current, sortOpt)
}

// stringSlicesEqual compares string slices, order doesn't matter.
func stringSlicesEqual(desired, current []string) bool {
	if len(desired) != len(current) {
		return false
	}

	sortOpt := cmpopts.SortSlices(func(a, b string) bool { return a < b })
	return cmp.Equal(desired, current, sortOpt)
}
