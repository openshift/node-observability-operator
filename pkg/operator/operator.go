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
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	nodeobservabilityv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	operatorconfig "github.com/openshift/node-observability-operator/pkg/operator/config"
	controller "github.com/openshift/node-observability-operator/pkg/operator/controller"
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
	clusterWideCli, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: GetOperatorScheme(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	if err := (&nodeobservabilitycontroller.NodeObservabilityReconciler{
		Client:            mgr.GetClient(),
		Scheme:            GetOperatorScheme(),
		Log:               ctrl.Log.WithName("controller.nodeobservability"),
		Namespace:         opCfg.OperatorNamespace,
		AgentImage:        opCfg.AgentImage,
		ClusterWideClient: clusterWideCli,
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
		AgentName: controller.AgentName,
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
