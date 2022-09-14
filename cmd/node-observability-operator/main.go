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

package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	securityv1 "github.com/openshift/api/security/v1"
	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	"go.uber.org/zap/zapcore"

	nodeobservabilityv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	machineconfigcontroller "github.com/openshift/node-observability-operator/pkg/operator/controller/machineconfig"
	nodeobservabilitycontroller "github.com/openshift/node-observability-operator/pkg/operator/controller/nodeobservability"
	nodeobservabilityrun "github.com/openshift/node-observability-operator/pkg/operator/controller/nodeobservabilityrun"
)

const (
	agentName = "node-observability-agent"
	// #nosec G101: Potential hardcoded credentials; path to token, not the content itself
	defaultTokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultCACertFile = "/var/run/secrets/openshift.io/certs/service-ca.crt"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("operator")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(nodeobservabilityv1alpha1.AddToScheme(scheme))
	utilruntime.Must(securityv1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(mcv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var operatorNamespace string
	var agentImage string
	var tokenFile string
	var caCertFile string

	flag.StringVar(&operatorNamespace, "operator-namespace", "node-observability-operator", "The node observability operator namespace.")
	flag.StringVar(&agentImage, "agent-image", "quay.io/node-observability-operator/node-observability-agent:latest", "The node observability agent container image to use.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&tokenFile, "token-file", defaultTokenFile, "The path of the service account token.")
	flag.StringVar(&caCertFile, "ca-cert-file", defaultCACertFile, "The path of the CA cert of the Agents' signing key pair.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. "+"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		TimeEncoder: zapcore.TimeEncoder(func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z"))
		}),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		setupLog.Error(err, "unable to read serviceaccount token")
		os.Exit(1)
	}
	ca, err := readCACert(caCertFile)
	if err != nil {
		setupLog.Error(err, "unable to read CA cert")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Logger:                 logger,
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "94c735b6.olm.openshift.io",
		Namespace:              operatorNamespace,
		// Use a non-caching client everywhere. The default split client does not
		// promise to invalidate the cache during writes (nor does it promise
		// sequential create/get coherence), and we have code which (probably
		// incorrectly) assumes a get immediately following a create/update will
		// return the updated resource. All client consumers will need audited to
		// ensure they are tolerant of stale data (or we need a cache or client that
		// makes stronger coherence guarantees).
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime#hdr-Clients_and_Caches
		NewClient: func(_ cache.Cache, config *rest.Config, options client.Options, _ ...client.Object) (client.Client, error) {
			return client.New(config, options)
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	// KubeAPI client to be used only in order to get the configmap kubelet-serving-ca
	// from NS openshift-config-managed.
	// The clusterWideCli is needed because ConfigMaps are namespaced resources, and in the context
	// of a namespaced operator, the operator only looks for the namespaced resources in its own namespace
	// see https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.1/pkg/manager#Options => Namespace
	clusterWideCli, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to create a client")
		os.Exit(1)
	}

	if err := (&nodeobservabilitycontroller.NodeObservabilityReconciler{
		Client:            mgr.GetClient(),
		ClusterWideClient: clusterWideCli,
		Scheme:            mgr.GetScheme(),
		Log:               ctrl.Log.WithName("controller.nodeobservability"),
		Namespace:         operatorNamespace,
		AgentImage:        agentImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "nodeobservability")
		os.Exit(1)
	}

	if err := machineconfigcontroller.New(mgr).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "nodeobservabilitymachineconfig")
		os.Exit(1)
	}

	if err := (&nodeobservabilityrun.NodeObservabilityRunReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Log:       ctrl.Log.WithName("controller.nodeobservabilityrun"),
		Namespace: operatorNamespace,
		AgentName: agentName,
		AuthToken: token,
		CACert:    ca,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "nodeobservabilityrun")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func readCACert(caCertFile string) (*x509.CertPool, error) {
	content, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	if len(content) <= 0 {
		return nil, fmt.Errorf("%s is empty", caCertFile)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(content) {
		return nil, fmt.Errorf("unable to add certificates into caCertPool: %v", err)

	}
	return caCertPool, nil
}
