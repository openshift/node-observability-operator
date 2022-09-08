package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	podName           = "node-observability-agent"
	socketName        = "socket"
	socketPath        = "/var/run/crio/crio.sock"
	socketMountPath   = "/var/run/crio/crio.sock"
	kbltCAMountPath   = "/var/run/secrets/kubelet-serving-ca/"
	kbltCAMountedFile = "ca-bundle.crt"
	kbltCAName        = "kubelet-ca"
	defaultScheduler  = "default-scheduler"
	daemonSetName     = "node-observability-ds"
	certsName         = "certs"
	certsMountPath    = "/var/run/secrets/openshift.io/certs"
)

// ensureDaemonSet ensures that the daemonset exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// daemonset and an error when relevant
func (r *NodeObservabilityReconciler) ensureDaemonSet(ctx context.Context, nodeObs *v1alpha1.NodeObservability, sa *corev1.ServiceAccount, ns string) (*appsv1.DaemonSet, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: daemonSetName}
	desired := r.desiredDaemonSet(nodeObs, sa, ns)
	if err := controllerutil.SetControllerReference(nodeObs, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set the controller reference for daemonset: %w", err)
	}

	current, err := r.currentDaemonSet(ctx, nameSpace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get daemonset %s/%s due to: %w", nameSpace.Namespace, nameSpace.Namespace, err)
	} else if err != nil && errors.IsNotFound(err) {

		// create daemon since it doesn't exist
		err := r.createConfigMap(ctx, nodeObs, ns)
		if err != nil {
			return nil, fmt.Errorf("failed to create the configmap for kubelet-serving-ca: %w", err)
		}

		if err := r.createDaemonSet(ctx, desired); err != nil {
			return nil, err
		}
		return r.currentDaemonSet(ctx, nameSpace)
	}

	updated, err := r.updateDaemonset(ctx, current, desired)
	if err != nil {
		return nil, err
	}

	if updated {
		current, err = r.currentDaemonSet(ctx, nameSpace)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing daemonset %s/%s: %w", nameSpace.Namespace, nameSpace.Name, err)
		}
	}
	return current, nil
}

// currentDaemonSet check if the daemonset exists
func (r *NodeObservabilityReconciler) currentDaemonSet(ctx context.Context, nameSpace types.NamespacedName) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, nameSpace, ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// createDaemonSet creates the serviceaccount
func (r *NodeObservabilityReconciler) createDaemonSet(ctx context.Context, ds *appsv1.DaemonSet) error {
	if err := r.Create(ctx, ds); err != nil {
		return fmt.Errorf("failed to create daemonset %s/%s: %w", ds.Namespace, ds.Name, err)
	}
	r.Log.V(1).Info("created daemonset", "ds.namespace", ds.Namespace, "ds.name", ds.Name)
	return nil
}

func (r *NodeObservabilityReconciler) updateDaemonset(ctx context.Context, current, desired *appsv1.DaemonSet) (bool, error) {
	updatedDS := current.DeepCopy()
	updated := false

	if !cmp.Equal(current.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		updatedDS.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
		updated = true
	}

	// if the desired and current daemonset container are not the same then just update
	if len(desired.Spec.Template.Spec.Containers) != len(updatedDS.Spec.Template.Spec.Containers) {
		updatedDS.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
		updated = true
	} else {
		// for each of the container in the desired daemonset ensure the corresponding container in the current matches
		for _, desiredContainer := range desired.Spec.Template.Spec.Containers {
			foundIndex := -1
			for i, currentContainer := range updatedDS.Spec.Template.Spec.Containers {
				if currentContainer.Name == desiredContainer.Name {
					foundIndex = i
					break
				}
			}
			if foundIndex < 0 {
				return false, fmt.Errorf("daemonset %s does not have a container with the name %s", current.Name, desiredContainer.Name)
			}

			if changed := hasContainerChanged(updatedDS.Spec.Template.Spec.Containers[foundIndex], desiredContainer); changed {
				updatedDS.Spec.Template.Spec.Containers[foundIndex] = desiredContainer
				updated = true
			}
		}
	}

	if haveVolumesChanged(updatedDS.Spec.Template.Spec.Volumes, desired.Spec.Template.Spec.Volumes) {
		updatedDS.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes
		updated = true
	}

	if updated {
		err := r.Update(ctx, updatedDS)
		if err != nil {
			return false, fmt.Errorf("failed to update existing daemonset %s: %w", updatedDS.Name, err)
		}
		return true, nil
	}

	return false, nil
}

// desiredDaemonSet returns a DaemonSet object
func (r *NodeObservabilityReconciler) desiredDaemonSet(nodeObs *v1alpha1.NodeObservability, sa *corev1.ServiceAccount, ns string) *appsv1.DaemonSet {
	ls := labelsForNodeObservability(nodeObs.Name)
	tgp := int64(30)
	vst := corev1.HostPathSocket
	privileged := true

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetName,
			Namespace: ns,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           r.AgentImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            podName,
							Command:         []string{"node-observability-agent"},
							Args: []string{
								"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token",
								"--storage=/run/node-observability",
								fmt.Sprintf("--caCertFile=%s%s", kbltCAMountPath, kbltCAMountedFile),
							},
							Resources:                corev1.ResourceRequirements{},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Env: []corev1.EnvVar{
								{
									Name: "NODE_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.hostIP",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: socketMountPath,
									Name:      socketName,
									ReadOnly:  false,
								},
								{
									MountPath: kbltCAMountPath,
									Name:      kbltCAName,
									ReadOnly:  true,
								},
							},
						},
						{
							Name:            "kube-rbac-proxy",
							Image:           "gcr.io/kubebuilder/kube-rbac-proxy:v0.11.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--secure-listen-address=0.0.0.0:8443",
								"--upstream=http://127.0.0.1:9000/",
								fmt.Sprintf("--tls-cert-file=%s/tls.crt", certsMountPath),
								fmt.Sprintf("--tls-private-key-file=%s/tls.key", certsMountPath),
								"--logtostderr=true",
								"--v=2",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      certsName,
									MountPath: certsMountPath,
									ReadOnly:  true,
								},
							},
						},
					},
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 defaultScheduler,
					ServiceAccountName:            sa.Name,
					TerminationGracePeriodSeconds: &tgp,
					Volumes: []corev1.Volume{
						{
							Name: socketName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: socketPath,
									Type: &vst,
								},
							},
						},
						{
							Name: kbltCAName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: nodeObs.Name,
									},
								},
							},
						},
						{
							Name: certsName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
								},
							},
						},
					},
					NodeSelector: nodeObs.Spec.NodeSelector,
				},
			},
		},
	}
	return ds
}

// labelsForNodeObservability returns the labels for selecting the resources
// belonging to the given node observability CR name.
func labelsForNodeObservability(name string) map[string]string {
	return map[string]string{"app": "nodeobservability", "nodeobs_cr": name}
}

// haveVolumesChanged if the current volumes differs from the desired volumes
func haveVolumesChanged(current []corev1.Volume, desired []corev1.Volume) bool {
	if len(current) != len(desired) {
		return true
	}
	for i := 0; i < len(current); i++ {
		cv := current[i]
		dv := desired[i]
		if cv.Name != dv.Name {
			return true
		}
		if dv.Secret != nil {
			if cv.Secret == nil {
				return true
			}
			if cv.Secret.SecretName != dv.Secret.SecretName {
				return true
			}
		}
	}
	return false
}

// hasContainerChanged checks if the current container differs from the
// desired container
func hasContainerChanged(current, desired corev1.Container) bool {
	if current.Image != desired.Image {
		return true
	}
	if !cmp.Equal(current.Args, desired.Args) {
		return true
	}

	if hasSecurityContextChanged(current.SecurityContext, desired.SecurityContext) {
		return true
	}

	if len(current.Env) != len(desired.Env) {
		return true
	}
	currentEnvs := indexedContainerEnv(current.Env)
	for _, e := range desired.Env {
		if ce, ok := currentEnvs[e.Name]; !ok {
			return true
		} else if !cmp.Equal(ce, e) {
			return true
		}
	}

	if len(current.VolumeMounts) != len(desired.VolumeMounts) {
		return true
	}

	for i := 0; i < len(current.VolumeMounts); i++ {
		cvm := current.VolumeMounts[i]
		dvm := desired.VolumeMounts[i]
		if cvm.Name != dvm.Name || cvm.MountPath != dvm.MountPath {
			return true
		}
	}

	return false
}

func indexedContainerEnv(envs []corev1.EnvVar) map[string]corev1.EnvVar {
	indexed := make(map[string]corev1.EnvVar)
	for _, e := range envs {
		indexed[e.Name] = e
	}
	return indexed
}

func hasSecurityContextChanged(current, desired *corev1.SecurityContext) bool {
	if desired == nil {
		return false
	}

	if current == nil {
		return true
	}

	if desired.Capabilities != nil {
		if current.Capabilities == nil {
			return true
		}

		cmpCapabilities := cmpopts.SortSlices(func(a, b corev1.Capability) bool { return a < b })
		if !cmp.Equal(desired.Capabilities.Add, current.Capabilities.Add, cmpCapabilities) {
			return true
		}

		if !cmp.Equal(desired.Capabilities.Drop, current.Capabilities.Drop, cmpCapabilities) {
			return true
		}
	}

	if !equalBoolPtr(current.RunAsNonRoot, desired.RunAsNonRoot) {
		return true
	}

	if !equalBoolPtr(current.Privileged, desired.Privileged) {
		return true
	}
	if !equalBoolPtr(current.AllowPrivilegeEscalation, desired.AllowPrivilegeEscalation) {
		return true
	}

	if desired.SeccompProfile != nil {
		if current.SeccompProfile == nil {
			return true
		}
		if desired.SeccompProfile.Type != "" && desired.SeccompProfile.Type != current.SeccompProfile.Type {
			return true
		}
	}
	return false
}

func equalBoolPtr(current, desired *bool) bool {
	if desired == nil {
		return true
	}

	if current == nil {
		return false
	}

	if *current != *desired {
		return false
	}
	return true
}
