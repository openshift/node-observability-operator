#!/bin/bash
oc patch nodeobservability/nodeobservability-sample --patch '{"metadata":{"finalizers": []}}' --type=merge
oc delete nodeobservability nodeobservability-sample
oc delete scc node-observability-scc
oc delete clusterrole node-observability-cr
oc delete clusterrolebinding node-observability-crb
kustomize build config/default/ | oc delete -f -
oc delete role node-observability-operator-manager-role
