/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeobservabilitycontroller

import (
	"context"
	"fmt"

	"github.com/openshift/node-observability-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilclock "k8s.io/apimachinery/pkg/util/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
)

const (
	terminating = "Terminating"
)

var (
	clock utilclock.Clock = utilclock.RealClock{}
)

// NodeObservabilityReconciler reconciles a NodeObservability object
type NodeObservabilityReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// Used to inject errors for testing
	Err ErrTestObject
}

//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodeobservability.olm.openshift.io,resources=nodeobservabilities/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=list;get;create;watch;
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=list;get;create;watch;
//+kubebuilder:rbac:groups=core,resources=serviceaccount,verbs=list;get;create;watch;
//+kubebuilder:rbac:groups=rbac,resources=clusterrole,verbs=list;get;create;watch;
//+kubebuilder:rbac:groups=rbac,resources=clusterrolebinding,verbs=list;get;create;watch;
//+kubebuilder:rbac:groups=security,resources=securitycontextconstraint,verbs=list;get;create;watch;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeObservability object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *NodeObservabilityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Fetch the NodeObservability instance
	nodeObs := &v1alpha1.NodeObservability{}
	err := r.Get(ctx, req.NamespacedName, nodeObs)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("NodeObservability resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get NodeObservability")
		return ctrl.Result{}, err
	}
	r.Log.Info(fmt.Sprintf("NodeObservability resource found : Namespace %s : Name %s ", req.NamespacedName.Namespace, nodeObs.Name))

	// For the pods to deploy on each node and execute the crio & kubelet script we need the following
	// - custom scc (mainly allowHostPathDirPlugin set to true)
	// - serviceaccount
	// - secret
	// - clusterrole (use the scc)
	// - clusterrolebinding (bind the sa to the role)

	// ensure scc
	haveSCC, scc, err := r.ensureSecurityContextConstraints(ctx, nodeObs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure securitycontectconstraints : %w", err)
	} else if !haveSCC {
		return reconcile.Result{}, fmt.Errorf("failed to get securitycontextconstraints")
	}
	r.Log.Info(fmt.Sprintf("SecurityContextConstraints : %s", scc.Name))

	// ensure secret
	haveSecret, secret, err := r.ensureSecret(ctx, nodeObs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure secret : %w", err)
	} else if !haveSecret {
		return reconcile.Result{}, fmt.Errorf("failed to get secret")
	}
	r.Log.Info(fmt.Sprintf("Secret : %s", secret.Name))

	// ensure serviceaccount with the secret
	haveSA, sa, err := r.ensureServiceAccount(ctx, nodeObs, secret)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure serviceaccount : %w", err)
	} else if !haveSA {
		return reconcile.Result{}, fmt.Errorf("failed to get serviceaccount")
	}
	r.Log.Info(fmt.Sprintf("ServiceAccount : %s", sa.Name))

	// check clusterrole
	haveCR, cr, err := r.ensureClusterRole(ctx, nodeObs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure clusterrole : %w", err)
	} else if !haveCR {
		return reconcile.Result{}, fmt.Errorf("failed to get clusterrole")
	}
	r.Log.Info(fmt.Sprintf("ClusterRole : %s", cr.Name))

	// check clusterolebinding with serviceaccount
	haveCRB, crb, err := r.ensureClusterRoleBinding(ctx, nodeObs, sa.Name)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure clusterrolebinding : %w", err)
	} else if !haveCRB {
		return reconcile.Result{}, fmt.Errorf("failed to get clusterrolebinding")
	}
	r.Log.Info(fmt.Sprintf("ClusterRoleBinding : %s", crb.Name))

	// check daemonset
	haveDS, ds, err := r.ensureDaemonSet(ctx, nodeObs, sa)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure daemonset : %w", err)
	} else if !haveDS {
		return reconcile.Result{}, fmt.Errorf("failed to get daemonset")
	}
	r.Log.Info(fmt.Sprintf("DaemonSet : %s", ds.Name))

	// check the pods that are deployed against the daemonset count
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(nodeObs.Namespace),
		client.MatchingLabels(labelsForNodeObservability(nodeObs.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		r.Log.Error(err, "Failed to list pods", "NodeObservability.Namespace", nodeObs.Namespace, "NodeObservability.Name", nodeObs.Name)
		return ctrl.Result{}, err
	}

	count := 0

	for x, pod := range podList.Items {
		r.Log.Info(fmt.Sprintf("Pod phase status %d %s", x, pod.Status.Phase))
		if pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != terminating {
			count++
		}
	}

	r.Log.Info("Updating status")
	nodeObs.Status.Count = len(podList.Items)
	now := metav1.NewTime(clock.Now())
	nodeObs.Status.LastUpdate = &now
	err = r.Status().Update(ctx, nodeObs)
	if err != nil {
		r.Log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}
	r.Log.Info(fmt.Sprintf("Status updated : %d", count))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeObservabilityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodeObservability{}).
		Owns(&appsv1.DaemonSet{}).
		Complete(r)
}
