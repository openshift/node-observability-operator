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

package operator

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	nodeobservabilityv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	operatorconfig "github.com/openshift/node-observability-operator/pkg/operator/config"
	opctrl "github.com/openshift/node-observability-operator/pkg/operator/controller"
	caconfigmapcontroller "github.com/openshift/node-observability-operator/pkg/operator/controller/ca-configmap"
	machineconfigcontroller "github.com/openshift/node-observability-operator/pkg/operator/controller/machineconfig"
	nodeobservabilitycontroller "github.com/openshift/node-observability-operator/pkg/operator/controller/nodeobservability"
	nodeobservabilityrun "github.com/openshift/node-observability-operator/pkg/operator/controller/nodeobservabilityrun"
)

// Operator hold the manager resource.
// for the nodeobservability opreator.
type Operator struct {
	manager manager.Manager
}

// New creates a new operator from cliCfg and opCfg.
func New(cliCfg *rest.Config, opCfg *operatorconfig.Config) (*Operator, error) {
	token, err := os.ReadFile(opCfg.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read serviceaccount token: %w", err)
	}
	ca, err := readCACert(opCfg.CaCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	config := ctrl.GetConfigOrDie()
	// Use a non-caching client everywhere. The default split client does not
	// promise to invalidate the cache during writes (nor does it promise
	// sequential create/get coherence), and we have code which (probably
	// incorrectly) assumes a get immediately following a create/update will
	// return the updated resource. All client consumers will need audited to
	// ensure they are tolerant of stale data (or we need a cache or client that
	// makes stronger coherence guarantees).
	// https://pkg.go.dev/sigs.k8s.io/controller-runtime#hdr-Clients_and_Caches
	newNoCacheClientFunc := func(_ cache.Cache, config *rest.Config, options client.Options, _ ...client.Object) (client.Client, error) {
		return client.New(config, options)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Logger:                 ctrl.Log.WithName("operator manager"),
		Scheme:                 GetOperatorScheme(),
		MetricsBindAddress:     opCfg.MetricsBindAddress,
		Port:                   9443,
		HealthProbeBindAddress: opCfg.HealthProbeBindAddress,
		LeaderElection:         opCfg.EnableLeaderElection,
		LeaderElectionID:       "94c735b6.olm.openshift.io",
		Namespace:              opCfg.OperatorNamespace,
		NewClient:              newNoCacheClientFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("failed to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("failed to set up ready check: %w", err)
	}
	// Cluster is a runnable managed by controller-runtime's manager
	// with its own client and cache.
	// We deliberately use a dedicated cache for the watches of ca-configmap-controller
	// to avoid the unnecessary cluster level permissions.
	// ca-configmap-controller needs a namespace different from the operator's (source namespace)
	// which forces the usage of the multinamespace shared cache of the manager.
	// The additional namespace in turn obliges all the other controllers to have the rights
	// to watch in it which results into cluster level permissions for all the operands:
	// daemonset, serviceaccounts, services, since the local role cannot be created in another namespace via OLM.
	// Inspired by https://github.com/kubernetes-sigs/controller-runtime/blob/master/designs/move-cluster-specific-code-out-of-manager.md
	cluster, err := cluster.New(config, func(opts *cluster.Options) {
		opts.NewClient = newNoCacheClientFunc
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime cluster: %w", err)
	}

	if err := mgr.Add(cluster); err != nil {
		return nil, fmt.Errorf("failed to add controller-runtime cluster to manager: %w", err)
	}
	// Create and register the CA config map controller with the operator manager.
	if _, err := caconfigmapcontroller.New(mgr, caconfigmapcontroller.Config{
		SourceNamespace: opctrl.SourceKubeletCAConfigMapNamespace,
		TargetNamespace: opCfg.OperatorNamespace,
		CAConfigMapName: opctrl.KubeletCAConfigMapName,
		Cluster:         cluster,
	}); err != nil {
		return nil, fmt.Errorf("failed to create CA config map controller controller: %w", err)
	}

	if err := (&nodeobservabilitycontroller.NodeObservabilityReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Log:        ctrl.Log.WithName("controller.nodeobservability"),
		Namespace:  opCfg.OperatorNamespace,
		AgentImage: opCfg.AgentImage,
	}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("failed to create nodeobservability controller: %w", err)
	}
	if err := machineconfigcontroller.New(mgr).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("failed to create nodeobservabilitymachineconfig controller: %w", err)
	}

	if err := (&nodeobservabilityrun.NodeObservabilityRunReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Log:       ctrl.Log.WithName("controller.nodeobservabilityrun"),
		Namespace: opCfg.OperatorNamespace,
		AgentName: opctrl.AgentName,
		AuthToken: token,
		CACert:    ca,
	}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("failed to create nodeobservabilityrun controller: %w", err)
	}

	if opCfg.EnableWebhook {
		if err = (&nodeobservabilityv1alpha1.NodeObservability{}).SetupWebhookWithManager(mgr); err != nil {
			return nil, fmt.Errorf("failed to create webhook nodeobservability version v1alpha1: %w", err)
		}
		if err = (&nodeobservabilityv1alpha1.NodeObservabilityMachineConfig{}).SetupWebhookWithManager(mgr); err != nil {
			return nil, fmt.Errorf("failed to create webhook nodeobservabilitymachineconfig version v1alpha1: %w", err)
		}
		if err = (&nodeobservabilityv1alpha2.NodeObservability{}).SetupWebhookWithManager(mgr); err != nil {
			return nil, fmt.Errorf("failed to create webhook nodeobservability version v1alpha2: %w", err)
		}
		if err = (&nodeobservabilityv1alpha2.NodeObservabilityMachineConfig{}).SetupWebhookWithManager(mgr); err != nil {
			return nil, fmt.Errorf("failed to create webhook nodeobservabilitymachineconfig version v1alpha2: %w", err)
		}
	}
	//+kubebuilder:scaffold:builder

	return &Operator{
		manager: mgr,
	}, nil
}

// Start starts the operator synchronously until a message is received from ctx.
func (o *Operator) Start(ctx context.Context) error {
	return o.manager.Start(ctx)
}
func readCACert(caCertFile string) (*x509.CertPool, error) {
	content, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	if len(content) <= 0 {
		return nil, fmt.Errorf("%s is empty", caCertFile)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(content) {
		return nil, fmt.Errorf("unable to add certificates into caCertPool: %w", err)

	}
	return caCertPool, nil
}
