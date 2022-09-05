package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	srcKbltCAConfigMapName      = "kubelet-serving-ca"
	srcKbltCAConfigMapNameSpace = "openshift-config-managed"
)

func (r *NodeObservabilityReconciler) createConfigMap(ctx context.Context, nodeObs *v1alpha1.NodeObservability, ns string) error {
	kbltCACM := &corev1.ConfigMap{}
	kbltCACMName := types.NamespacedName{
		Name:      srcKbltCAConfigMapName,
		Namespace: srcKbltCAConfigMapNameSpace,
	}
	// Use the clusterWide client in order to get the configmap from openshift-config-managed namespace
	// As the default client will only look for configmaps inside the namespace
	if err := r.ClusterWideClient.Get(ctx, kbltCACMName, kbltCACM); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting source configmap %q, %w", kbltCACMName, err)
		}
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
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       nodeObs.Name,
					Kind:       nodeObs.Kind,
					UID:        nodeObs.UID,
					APIVersion: nodeObs.APIVersion,
				},
			},
		},
		Data: map[string]string{
			kbltCAMountedFile: kbltCACM.Data[kbltCAMountedFile],
		},
	}

	if err := r.Get(ctx, configMapName, &corev1.ConfigMap{}); err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("error getting target configmap %s, %w", configMapName, err)
	}

	if err := r.Create(ctx, configMap); err != nil {
		return fmt.Errorf("failed to create target configmap %s: %w", configMapName, err)
	}

	r.Log.Info("created configmap", "name", configMapName.Namespace, "name", configMapName.Name)
	return nil
}
