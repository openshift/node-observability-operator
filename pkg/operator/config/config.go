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

package config

const (
	DefaultOperatorNamespace    = "node-observability-operator"
	DefaultAgentImage           = "quay.io/node-observability-operator/node-observability-agent:latest"
	DefaultMetricsAddr          = ":8080"
	DefaultEnableWebhook        = true
	DefaultHealthProbeAddr      = ":8081"
	DefaultEnableLeaderElection = false
	// #nosec G101: Potential hardcoded credentials; path to token, not the content itself
	DefaultTokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	DefaultCACertFile = "/var/run/secrets/openshift.io/certs/service-ca.crt"
)

// Config is configuration of the operator.
type Config struct {

	// OperatorNamespace is the namespace that the operator is deployed in.
	OperatorNamespace string

	// The node observability agent container image to use.
	AgentImage string

	// MetricsBindAddress is the TCP address that the operator should bind to for
	// serving prometheus metrics. It can be set to "0" to disable the metrics serving.
	MetricsBindAddress string

	// HealthProbeBindAddress is the TCP address that the operator should bind to for
	// serving health probes (readiness and liveness).
	HealthProbeBindAddress string

	// The path of the service account token.
	TokenFile string

	// The path of the CA cert of the Agents' signing key pair.
	CaCertFile string

	// EnableWebhook is the flag indicating if the webhook server should be started.
	EnableWebhook bool

	// EnableLeaderElection enables the controller runtime's leader election.
	EnableLeaderElection bool
}
