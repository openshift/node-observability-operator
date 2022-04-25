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

func (r *NodeObservabilityReconciler) createConfigMap(nodeObs *v1alpha1.NodeObservability) (bool, error) {
	kbltCACM := &corev1.ConfigMap{}
	kbltCACMName := types.NamespacedName{
		Name:      srcKbltCAConfigMapName,
		Namespace: srcKbltCAConfigMapNameSpace,
	}
	// Use the clusterWide client in order to get the configmap from openshift-config-managed namespace
	// As the default client will only look for configmaps inside the namespace
	if err := r.ClusterWideClient.Get(context.TODO(), kbltCACMName, kbltCACM); err != nil {
		if errors.IsNotFound(err) {
			return false, fmt.Errorf("unable to find configMap %v: %w", kbltCACMName, err)
		}
		return false, fmt.Errorf("error getting configMap %v, %w", kbltCACMName, err)
	}

	// Copy the configmap into the namespace
	caChain := kbltCACM.Data[kbltCAMountedFile]
	newDataMap := make(map[string]string, 1)
	newDataMap[kbltCAMountedFile] = caChain

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeObs.Name,
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
		Data: newDataMap,
	}

	if err := r.Create(context.TODO(), configMap); err != nil {
		return false, fmt.Errorf("failed to create configMap %s/%s: %w", configMap.Namespace, configMap.Name, err)
	}
	r.Log.Info("created ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
	return true, nil
}
