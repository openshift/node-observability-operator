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
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/openshift/node-observability-operator/pkg/operator"
	operatorconfig "github.com/openshift/node-observability-operator/pkg/operator/config"
	"github.com/openshift/node-observability-operator/pkg/version"
)

var (
	opCfg operatorconfig.Config
)

func main() {
	flag.StringVar(&opCfg.OperatorNamespace, "operator-namespace", operatorconfig.DefaultOperatorNamespace, "The node observability operator namespace.")
	flag.StringVar(&opCfg.AgentImage, "agent-image", operatorconfig.DefaultAgentImage, "The node observability agent container image to use.")
	flag.StringVar(&opCfg.MetricsBindAddress, "metrics-bind-address", operatorconfig.DefaultMetricsAddr, "The address the metric endpoint binds to.")
	flag.StringVar(&opCfg.HealthProbeBindAddress, "health-probe-bind-address", operatorconfig.DefaultHealthProbeAddr, "The address the probe endpoint binds to.")
	flag.StringVar(&opCfg.TokenFile, "token-file", operatorconfig.DefaultTokenFile, "The path of the service account token.")
	flag.StringVar(&opCfg.CaCertFile, "ca-cert-file", operatorconfig.DefaultCACertFile, "The path of the CA cert of the Agents' signing key pair.")
	flag.BoolVar(&opCfg.EnableLeaderElection, "leader-elect", operatorconfig.DefaultEnableLeaderElection, "Enable leader election for controller manager. "+"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&opCfg.EnableWebhook, "enable-webhook", operatorconfig.DefaultEnableWebhook, "Enable the webhook server(s). Defaults to true.")

	opts := zap.Options{
		TimeEncoder: zapcore.TimeEncoder(func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z"))
		}),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	setupLog := ctrl.Log.WithName("setup")
	ctrl.Log.Info("build info", "commit", version.COMMIT)
	ctrl.Log.Info("using operator namespace", "namespace", opCfg.OperatorNamespace)
	ctrl.Log.Info("using AgentImage image", "image", opCfg.AgentImage)

	kubeConfig := ctrl.GetConfigOrDie()
	op, err := operator.New(kubeConfig, &opCfg)
	if err != nil {
		setupLog.Error(err, "failed to create NodeObservability operator")
		os.Exit(1)
	}
	setupLog.Info("starting NodeObservability operator")
	if err := op.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "failed to start NodeObservability operator")
		os.Exit(1)
	}
}
