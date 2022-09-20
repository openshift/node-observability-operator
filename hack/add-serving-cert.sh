#!/usr/bin/env bash

# Meant to secure the communication between the API and the webhook server's endpoint
# using OpenShift's service serving certificate

set -e

usage() {
  cat <<EOF
Make the service serving certificates and add the CA bundle to the validating webhook's client config.
usage: ${0} [OPTIONS]
The following flags are required.
    --namespace        Namespace where webhook server resides.
    --service          Service name of webhook server.
    --secret           Secret name for CA certificate and server certificate/key pair.
    --webhook          Validating webhook config name to be injected with CA.
    --crd              CRD name to be injected with CA.
EOF
  exit 1
}

while [ $# -gt 0 ]; do
  case ${1} in
      --service)
          service="$2"
          shift
          ;;
      --webhook)
          webhook="$2"
          shift
          ;;
      --crd)
          crd="$2"
          shift
          ;;
      --secret)
          secret="$2"
          shift
          ;;
      --namespace)
          namespace="$2"
          shift
          ;;
      *)
          usage
          ;;
  esac
  shift
done

[ -z "${service}" ] && echo "ERROR: --service flag is required" && exit 1
[ -z "${secret}" ] && echo "ERROR: --secret flag is required" && exit 1
[ -z "${namespace}" ] && echo "ERROR: --namespace flag is required" && exit 1

if [ -z "${webhook}" ] && [ -z "${crd}" ]; then
    echo "ERROR: --webhook or --crd flag required"
    exit 1
fi

if [ ! -x "$(command -v oc)" ]; then
  echo "ERROR: oc not found"
  exit 1
fi

oc -n "${namespace}" annotate service "${service}" "service.beta.openshift.io/serving-cert-secret-name=${secret}" --overwrite=true

if [ -n "${webhook}" ]; then
    oc annotate validatingwebhookconfigurations "${webhook}" "service.beta.openshift.io/inject-cabundle=true" --overwrite=true
fi

if [ -n "${crd}" ]; then
    oc annotate crd "${crd}" "service.beta.openshift.io/inject-cabundle=true" --overwrite=true
fi
