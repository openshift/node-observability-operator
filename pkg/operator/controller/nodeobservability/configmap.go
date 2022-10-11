package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
)

const (
	srcKbltCAConfigMapName      = "kubelet-serving-ca"
	srcKbltCAConfigMapNameSpace = "openshift-config-managed"
)

func (r *NodeObservabilityReconciler) createConfigMap(ctx context.Context, nodeObs *v1alpha2.NodeObservability, ns string) error {
	kbltCACM := &corev1.ConfigMap{}
	kbltCACMName := types.NamespacedName{
		Name:      srcKbltCAConfigMapName,
		Namespace: srcKbltCAConfigMapNameSpace,
	}
	// Use the clusterWide client in order to get the configmap from openshift-config-managed namespace
	// As the default client will only look for configmaps inside the namespace
	if err := r.ClusterWideClient.Get(ctx, kbltCACMName, kbltCACM); err != nil {
		return fmt.Errorf("error getting source configmap %q, %w", kbltCACMName, err)
	}

	// Copy the configmap into the operator namespace
	configMapName := types.NamespacedName{
		Name:      nodeObs.Name,
		Namespace: ns,
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName.Name,
			Namespace: configMapName.Namespace,
		},
		Data: map[string]string{
			kbltCAMountedFile: kbltCACM.Data[kbltCAMountedFile],
		},
	}

	if err := controllerutil.SetControllerReference(nodeObs, configMap, r.Scheme); err != nil {
		return fmt.Errorf("failed to set the controller reference for target configmap %q: %w", configMapName, err)
	}

	if err := r.Client.Get(ctx, configMapName, &corev1.ConfigMap{}); err != nil {
		if errors.IsNotFound(err) {
			// create configmap since it is not found
			if err := r.Create(ctx, configMap); err != nil {
				return fmt.Errorf("failed to create target configmap %q: %w", configMapName, err)
			}
			r.Log.V(1).Info("created kubelet CA configmap", "configmap.namespace", configMapName.Namespace, "configmap.name", configMapName.Name)
		} else {
			return fmt.Errorf("error getting target configmap %q, %w", configMapName, err)
		}
	}

	r.Log.V(1).Info("verified kubelet CA configmap exists", "configmap.namespace", configMapName.Namespace, "configmap.name", configMapName.Name)
	return nil
}
