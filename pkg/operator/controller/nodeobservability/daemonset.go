package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	podName          = "node-observability-agent"
	socketName       = "socket"
	path             = "/var/run/crio/crio.sock"
	mountPath        = "/var/run/crio/crio.sock"
	defaultScheduler = "default-scheduler"
	daemonSetName    = "node-observability-ds"
	certsName        = "certs"
	certsMountPath   = "/var/run/secrets/openshift.io/certs"
)

// ensureDaemonSet ensures that the daemonset exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// daemonset and an error when relevant
func (r *NodeObservabilityReconciler) ensureDaemonSet(ctx context.Context, nodeObs *v1alpha1.NodeObservability, sa *corev1.ServiceAccount) (bool, *appsv1.DaemonSet, error) {
	nameSpace := types.NamespacedName{Namespace: nodeObs.Namespace, Name: daemonSetName}
	desired := r.desiredDaemonSet(nodeObs, sa)
	exist, current, err := r.currentDaemonSet(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get DaemonSet: %v", err)
	}
	if !exist {
		if err := r.createDaemonSet(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentDaemonSet(ctx, nameSpace)
	}
	return true, current, err
}

// currentDaemonSet check if the daemonset exists
func (r *NodeObservabilityReconciler) currentDaemonSet(ctx context.Context, nameSpace types.NamespacedName) (bool, *appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, nameSpace, ds); err != nil || r.Err.Set[dsObj] {
		if errors.IsNotFound(err) || r.Err.NotFound[dsObj] {
			return false, nil, nil
		}
		if r.Err.Set[dsObj] {
			err = fmt.Errorf("failed to get DaemonSet: simulated error")
		}
		return false, nil, err
	}
	return true, ds, nil
}

// createDaemonSet creates the serviceaccount
func (r *NodeObservabilityReconciler) createDaemonSet(ctx context.Context, ds *appsv1.DaemonSet) error {
	if err := r.Create(ctx, ds); err != nil {
		return fmt.Errorf("failed to create DaemonSet %s/%s: %w", ds.Namespace, ds.Name, err)
	}
	r.Log.Info("created DaemonSet", "DaemonSet.Namespace", ds.Namespace, "DaemonSet.Name", ds.Name)
	return nil
}

// desiredDaemonSet returns a DaemonSet object
func (r *NodeObservabilityReconciler) desiredDaemonSet(nodeObs *v1alpha1.NodeObservability, sa *corev1.ServiceAccount) *appsv1.DaemonSet {

	ls := labelsForNodeObservability(nodeObs.Name)
	tgp := int64(30)
	vst := corev1.HostPathSocket
	privileged := true

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetName,
			Namespace: nodeObs.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       nodeObs.Name,
					Kind:       nodeObs.Kind,
					UID:        nodeObs.UID,
					APIVersion: nodeObs.APIVersion,
				},
			},
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
							Image:                    nodeObs.Spec.Image,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							Name:                     podName,
							Command:                  []string{"node-observability-agent"},
							Args:                     []string{"--tokenFile=/var/run/secrets/kubernetes.io/serviceaccount/token", "--storage=/run"},
							Resources:                corev1.ResourceRequirements{},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Env: []corev1.EnvVar{{
								Name: "NODE_IP",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "status.hostIP",
									},
								},
							}},
							VolumeMounts: []corev1.VolumeMount{{
								MountPath: mountPath,
								Name:      socketName,
								ReadOnly:  false,
							}},
						},
						{
							Name:  "kube-rbac-proxy",
							Image: "gcr.io/kubebuilder/kube-rbac-proxy:v0.11.0",
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
									Path: path,
									Type: &vst,
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
					NodeSelector: nodeObs.Spec.Labels,
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
