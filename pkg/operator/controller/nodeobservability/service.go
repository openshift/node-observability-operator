package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
)

const (
	serviceName    = podName
	secretName     = podName
	injectCertsKey = "service.beta.openshift.io/serving-cert-secret-name"
	port           = 8443
	targetPort     = port
)

var (
	requestCerts = map[string]string{injectCertsKey: serviceName}
)

// ensureService ensures that the service exists
// Returns a Boolean value indicating whether it exists, a pointer to the
// service and an error when relevant
func (r *NodeObservabilityReconciler) ensureService(ctx context.Context, nodeObs *v1alpha1.NodeObservability) (bool, *corev1.Service, error) {
	nameSpace := types.NamespacedName{Namespace: nodeObs.Namespace, Name: serviceName}
	desired := r.desiredService(nodeObs)
	exist, current, err := r.currentService(ctx, nameSpace)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get Service: %v", err)
	}
	if !exist {
		if err := r.createService(ctx, desired); err != nil {
			return false, nil, err
		}
		return r.currentService(ctx, nameSpace)
	}
	return true, current, err
}

// currentService checks that the service exists
func (r *NodeObservabilityReconciler) currentService(ctx context.Context, nameSpace types.NamespacedName) (bool, *corev1.Service, error) {
	svc := &corev1.Service{}
	if err := r.Get(ctx, nameSpace, svc); err != nil || r.Err.Set[svcObj] {
		if errors.IsNotFound(err) || r.Err.NotFound[svcObj] {
			return false, nil, nil
		}
		if r.Err.Set[svcObj] {
			err = fmt.Errorf("failed to get Service: simulated error")
		}
		return false, nil, err
	}
	return true, svc, nil
}

// createService creates the service
func (r *NodeObservabilityReconciler) createService(ctx context.Context, svc *corev1.Service) error {
	if err := r.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create Service %s/%s: %w", svc.Namespace, svc.Name, err)
	}
	r.Log.Info("created Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
	return nil
}

// desiredService returns a service object
func (r *NodeObservabilityReconciler) desiredService(nodeObs *v1alpha1.NodeObservability) *corev1.Service {
	ls := labelsForNodeObservability(daemonSetName)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   nodeObs.Namespace,
			Name:        serviceName,
			Annotations: requestCerts,
			Labels:      ls,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       nodeObs.Name,
					Kind:       nodeObs.Kind,
					UID:        nodeObs.UID,
					APIVersion: nodeObs.APIVersion,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  ls,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       port,
					TargetPort: intstr.FromInt(targetPort),
				},
			},
		},
	}
	return svc
}
