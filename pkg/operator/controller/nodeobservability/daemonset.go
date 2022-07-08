package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
func (r *NodeObservabilityReconciler) ensureDaemonSet(ctx context.Context, nodeObs *v1alpha1.NodeObservability, sa *corev1.ServiceAccount, ns string) (bool, *appsv1.DaemonSet, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: daemonSetName}
	desired := r.desiredDaemonSet(nodeObs, sa, ns)
	if err := controllerutil.SetControllerReference(nodeObs, desired, r.Scheme); err != nil {
		return false, nil, fmt.Errorf("failed to set the controller reference for daemonset: %w", err)
	}
	exist, current, err := r.currentDaemonSet(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get DaemonSet: %w", err)
	}
	if !exist {
		cmExists, err := r.createConfigMap(ctx, nodeObs, ns)
		if err != nil {
			return false, nil, fmt.Errorf("failed to create the configMap for kubelet-serving-ca: %w", err)
		}
		if !cmExists {
			return false, nil, fmt.Errorf("failed to get the configMap for kubelet-serving-ca: %w", err)
		}
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
							Env: []corev1.EnvVar{{
								Name: "NODE_IP",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "status.hostIP",
									},
								},
							}},
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
