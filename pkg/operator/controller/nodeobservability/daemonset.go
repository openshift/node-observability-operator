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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	opctrl "github.com/openshift/node-observability-operator/pkg/operator/controller"
)

const (
	// nolint - ignore G101: not applicable
	podName           = "node-observability-agent"
	socketName        = "socket"
	socketPath        = "/var/run/crio/crio.sock"
	socketMountPath   = "/var/run/crio/crio.sock"
	kbltCAMountPath   = "/var/run/secrets/kubelet-serving-ca/"
	kbltCAMountedFile = "ca-bundle.crt"
	kbltCAName        = "kubelet-ca"
	defaultScheduler  = "default-scheduler"
	// nolint - ignore G101: not applicable
	obsoleteDaemonSetName = "node-observability-ds"
	// nolint - ignore G101: not applicable
	daemonSetName       = "node-observability-agent"
	certsName           = "certs"
	certsMountPath      = "/var/run/secrets/openshift.io/certs"
	kubeletCAAnnotation = "nodeobservability.olm.openshift.io/kubelet-ca-configmap-hash"
	dataVolumeName      = "profiledata"
	dataMountPath       = "/run/node-observability"
)

// ensureDaemonSet ensures that the daemonset exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// daemonset and an error when relevant
func (r *NodeObservabilityReconciler) ensureDaemonSet(ctx context.Context, nodeObs *v1alpha2.NodeObservability, sa *corev1.ServiceAccount, ns string, kubeletCAConfigMap *corev1.ConfigMap) (*appsv1.DaemonSet, error) {
	kubeletCAHash, err := buildMapHash(kubeletCAConfigMap.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to build hash of kubelet ca: %w", err)
	}
	desired := r.desiredDaemonSet(nodeObs, sa, ns, kubeletCAConfigMap.Name, kubeletCAHash)
	if err := controllerutil.SetControllerReference(nodeObs, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set the controller reference for daemonset: %w", err)
	}
	// migration logic for updated daemonset
	// TODO: remove this logic before going GA.
	if err := r.purgeObsoleteDaemonset(ctx, types.NamespacedName{Name: obsoleteDaemonSetName, Namespace: ns}); err != nil {
		return nil, fmt.Errorf("failed to purge obsolete daemonset due to %w", err)
	}
	nameSpace := types.NamespacedName{Namespace: ns, Name: daemonSetName}
	current, err := r.currentDaemonSet(ctx, nameSpace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get daemonset %q due to: %w", nameSpace, err)
	} else if err != nil && errors.IsNotFound(err) {
		// create daemon since it doesn't exist
		if err := r.createDaemonSet(ctx, desired); err != nil {
			return nil, fmt.Errorf("failed to create daemonset %q: %w", nameSpace, err)
		}
		r.Log.V(1).Info("created daemonset", "ds.namespace", nameSpace.Namespace, "ds.name", nameSpace.Name)

		return r.currentDaemonSet(ctx, nameSpace)
	}

	updated, err := r.updateDaemonset(ctx, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update the daemonset %q: %w", nameSpace, err)
	}

	if updated {
		current, err = r.currentDaemonSet(ctx, nameSpace)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing daemonset %q: %w", nameSpace, err)
		}
		r.Log.V(1).Info("successfully updated daemonset", "ds.name", nameSpace.Name, "ds.namespace", nameSpace.Namespace)
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
	return r.Create(ctx, ds)
}

func (r *NodeObservabilityReconciler) updateDaemonset(ctx context.Context, current, desired *appsv1.DaemonSet) (bool, error) {
	updatedDS := current.DeepCopy()
	updated := false

	if !cmp.Equal(current.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		updatedDS.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
		updated = true
	}

	if !cmp.Equal(current.Spec.Template.Labels, desired.Spec.Template.Labels) {
		updatedDS.Spec.Template.Labels = desired.Spec.Template.Labels
		updated = true
	}

	if !cmp.Equal(current.Spec.Template.Annotations, desired.Spec.Template.Annotations) {
		updatedDS.Spec.Template.Annotations = desired.Spec.Template.Annotations
		updated = true
	}

	if changed, updatedContainers := containersChanged(current.Spec.Template.Spec.Containers, desired.Spec.Template.Spec.Containers); changed {
		updatedDS.Spec.Template.Spec.Containers = updatedContainers
		updated = true
	}

	if changed, updatedVolumes := volumesChanged(updatedDS.Spec.Template.Spec.Volumes, desired.Spec.Template.Spec.Volumes); changed {
		updatedDS.Spec.Template.Spec.Volumes = updatedVolumes
		updated = true
	}

	if !cmp.Equal(current.Spec.Template.Spec.DNSPolicy, desired.Spec.Template.Spec.DNSPolicy) {
		updatedDS.Spec.Template.Spec.DNSPolicy = desired.Spec.Template.Spec.DNSPolicy
		updated = true
	}

	if updated {
		if err := r.Update(ctx, updatedDS); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// desiredDaemonSet returns a DaemonSet object
func (r *NodeObservabilityReconciler) desiredDaemonSet(nodeObs *v1alpha2.NodeObservability, sa *corev1.ServiceAccount, ns, kubeletCAConfigMapName, kubeletCAHash string) *appsv1.DaemonSet {
	ls := labelsForNodeObservability(nodeObs.Name)
	annotations := map[string]string{
		kubeletCAAnnotation: kubeletCAHash,
	}
	// profiling probe currently takes 30 seconds (default),
	// giving enough time to gracefully finish all the profiling requests
	tgp := int64(45)
	vst := corev1.HostPathSocket
	privileged := true

	var args []string

	if nodeObs.Spec.Type == "scripting" {
		args = []string{
			fmt.Sprintf("--storage=%s", dataMountPath),
			"--mode=scripting",
		}
	} else {
		args = []string{
			"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token",
			fmt.Sprintf("--storage=%s", dataMountPath),
			fmt.Sprintf("--caCertFile=%s%s", kbltCAMountPath, kbltCAMountedFile),
			"--mode=profiling",
		}
	}

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
					Labels:      ls,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           r.AgentImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            podName,
							Command:         []string{"node-observability-agent"},
							Args:            args,
							Resources:       corev1.ResourceRequirements{},
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
								{
									Name:  "EXECUTE_SCRIPT",
									Value: "/tmp/scripts/metrics.sh",
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
								{
									Name:      dataVolumeName,
									MountPath: dataMountPath,
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
										Name: kubeletCAConfigMapName,
									},
								},
							},
						},
						{
							Name: certsName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: opctrl.ServingCertSecretName,
								},
							},
						},
						{
							Name: dataVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
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

// purgeObsoleteDaemonset deletes the obsolete version of daemonset if present.
func (r *NodeObservabilityReconciler) purgeObsoleteDaemonset(ctx context.Context, name types.NamespacedName) error {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	if err := r.Client.Delete(context.TODO(), ds); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
