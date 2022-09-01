package nodeobservabilitycontroller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/google/go-cmp/cmp"
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
func (r *NodeObservabilityReconciler) ensureService(ctx context.Context, nodeObs *v1alpha1.NodeObservability, ns string) (*corev1.Service, error) {
	nameSpace := types.NamespacedName{Namespace: ns, Name: serviceName}

	desired := r.desiredService(nodeObs, ns)
	if err := controllerutil.SetControllerReference(nodeObs, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set the controller reference for service %q: %w", nameSpace.Name, err)
	}

	current, err := r.currentService(ctx, nameSpace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get service %s/%s due to: %w", nameSpace.Namespace, nameSpace.Name, err)
	} else if err != nil && errors.IsNotFound(err) {

		// creating service since it is not found
		if err := r.createService(ctx, desired); err != nil {
			return nil, err
		}
		log.FromContext(ctx).Info("successfully created service", "name", nameSpace.Name, "namespace", nameSpace.Namespace)
		return r.currentService(ctx, nameSpace)
	}

	// update service since it already exists
	return r.updateService(ctx, current, desired)
}

// currentService checks that the service exists
func (r *NodeObservabilityReconciler) currentService(ctx context.Context, nameSpace types.NamespacedName) (*corev1.Service, error) {
	svc := &corev1.Service{}
	if err := r.Get(ctx, nameSpace, svc); err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s due to: %w", nameSpace.Namespace, nameSpace.Name, err)
	}
	return svc, nil
}

// createService creates the service
func (r *NodeObservabilityReconciler) createService(ctx context.Context, svc *corev1.Service) error {
	if err := r.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create service %s/%s: %w", svc.Namespace, svc.Name, err)
	}
	r.Log.Info("created service", "service.Namespace", svc.Namespace, "service.Name", svc.Name)
	return nil
}

func (r *NodeObservabilityReconciler) updateService(ctx context.Context, current, desired *corev1.Service) (*corev1.Service, error) {
	updatedService := current.DeepCopy()
	var updated bool

	if !cmp.Equal(current.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		updatedService.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
		updated = true
	}

	if !portsMatch(updatedService.Spec.Ports, desired.Spec.Ports) {
		updatedService.Spec.Ports = desired.Spec.Ports
		updated = true
	}
	if !equality.Semantic.DeepEqual(updatedService.Spec.Selector, desired.Spec.Selector) {
		updatedService.Spec.Selector = desired.Spec.Selector
		updated = true
	}

	if updatedService.Spec.Type != desired.Spec.Type {
		updatedService.Spec.Type = desired.Spec.Type
		updated = true
	}

	if updatedService.Annotations == nil {
		updatedService.Annotations = make(map[string]string)
	}
	for annotationKey, annotationValue := range desired.Annotations {
		if currentAnnotationValue, ok := updatedService.Annotations[annotationKey]; !ok || currentAnnotationValue != annotationValue {
			updatedService.Annotations[annotationKey] = annotationValue
			updated = true
		}
	}

	if updated {
		return updatedService, r.Update(ctx, updatedService)
	}
	return updatedService, nil
}

// desiredService returns a service object
func (r *NodeObservabilityReconciler) desiredService(nodeObs *v1alpha1.NodeObservability, ns string) *corev1.Service {
	ls := labelsForNodeObservability(nodeObs.Name)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns,
			Name:        serviceName,
			Annotations: requestCerts,
			Labels:      ls,
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

type SortableServicePort []corev1.ServicePort

func (s SortableServicePort) Len() int {
	return len(s)
}

func (s SortableServicePort) Less(i, j int) bool {
	return strings.Compare(s[i].Name, s[j].Name) < 0
}

func (s SortableServicePort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func portsMatch(current, desired SortableServicePort) bool {
	if len(current) != len(desired) {
		return false
	}
	currentCopy := make(SortableServicePort, len(current))
	copy(currentCopy, current)
	sort.Sort(currentCopy)
	desiredCopy := make(SortableServicePort, len(desired))
	copy(desiredCopy, desired)
	sort.Sort(desiredCopy)

	for i := 0; i < len(currentCopy); i++ {
		c := currentCopy[i]
		d := desiredCopy[i]
		if c.Name != d.Name || c.Port != d.Port || c.TargetPort.IntVal != d.TargetPort.IntVal {
			return false
		}
	}
	return true
}
