# BUNDLE_VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the BUNDLE_VERSION as arg of the bundle target (e.g make bundle BUNDLE_VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export BUNDLE_VERSION=0.0.2)
BUNDLE_VERSION ?= 0.0.1

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# github.com/node-observability-operator-bundle:$BUNDLE_VERSION and github.com/node-observability-operator-catalog:$BUNDLE_VERSION.
IMAGE_TAG_BASE ?= quay.io/openshift/node-observability-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(BUNDLE_VERSION)

# INDEX_IMG defines the image:tag for the index build
INDEX_IMG ?= $(IMAGE_TAG_BASE)-index:v$(BUNDLE_VERSION)

# Image version to to build/push
IMG_VERSION ?= v0.0.1

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(IMG_VERSION)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.23

# CONTAINER_ENGINE defines which container executable to use
CONTAINER_ENGINE ?= podman

# VERBOSE used in testing for verbose output
VERBOSE ?=

ACK_GINKGO_DEPRECATIONS ?= 1.16.5

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

PACKAGE=github.com/openshift/node-observability-operator
MAIN_PACKAGE=$(PACKAGE)/cmd/node-observability-operator

BIN=bin/$(lastword $(subst /, ,$(MAIN_PACKAGE)))
BIN_DIR=$(shell pwd)/bin

GOBUILD_VERSION_ARGS = -ldflags "-X $(PACKAGE)/pkg/version.SHORTCOMMIT=$(SHORTCOMMIT) -X $(PACKAGE)/pkg/version.COMMIT=$(COMMIT)"

E2E_TIMEOUT ?= 1h

BUNDLE_DIR := bundle
BUNDLE_MANIFEST_DIR := $(BUNDLE_DIR)/manifests

OPERATOR_SDK_VERSION = v1.19.0

OPERATOR_SDK_BIN = $(shell pwd)/bin/operator-sdk
.PHONY: operator-sdk
operator-sdk:
	mkdir -p ${BIN_DIR};
ifeq ("$(wildcard $(OPERATOR_SDK_BIN))","")
	@{ \
	set -e ;\
	curl -Lk  https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64 > $(OPERATOR_SDK_BIN) ;\
	chmod u+x $(OPERATOR_SDK_BIN) ;\
	}
endif

CONTROLLER_GEN ?= go run sigs.k8s.io/controller-tools/cmd/controller-gen

KUSTOMIZE ?= go run sigs.k8s.io/kustomize/kustomize/v4

ENVTEST ?= go run sigs.k8s.io/controller-runtime/tools/setup-envtest

GOLANGCI_LINT_BIN ?= go run github.com/golangci/golangci-lint/cmd/golangci-lint

AUTH ?=

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."


.PHONY: test-e2e
test-e2e: 
	go test \
	$(GOBUILD_VERSION_ARGS) \
	-timeout $(E2E_TIMEOUT) \
	-count 1 \
	-v \
	-tags e2e \
	-run "$(TEST)" \
	./test/e2e
	
verify: lint
	hack/verify-bundle.sh
	hack/verify-gofmt.sh
	hack/verify-deps.sh
	hack/verify-generated.sh

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt -mod=vendor  ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet -mod=vendor ./...

.PHONY: test
test: manifests generate fmt vet ## Run tests.
	mkdir -p test/unit/results
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ${VERBOSE} -mod=vendor ./pkg/... -coverprofile test/unit/results/cover.out

##@ Build

GO=GO111MODULE=on GOFLAGS=-mod=vendor CGO_ENABLED=0 go

.PHONY: build-operator
build-operator: ## Build operator binary, no additional checks or code generation
	$(GO) build $(GOBUILD_VERSION_ARGS) -o $(BIN) $(MAIN_PACKAGE)

.PHONY: build
build: generate fmt vet build-operator ## Build manager binary.

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	$(GO) run -mod=vendor ./cmd/node-observability-operator/main.go

.PHONY: container-build
container-build: test ## Build image with the manager.
	${CONTAINER_ENGINE} build -t ${IMG} .

.PHONY: container-push
container-push: ## Push image with the manager.
	${CONTAINER_ENGINE} push ${IMG} $(AUTH)

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | oc apply -f -

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | oc  delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | oc apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | oc delete --ignore-not-found=$(ignore-not-found) -f -


.PHONY: bundle
bundle: operator-sdk manifests ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK_BIN) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK_BIN) generate bundle -q --overwrite=false --version $(BUNDLE_VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK_BIN) bundle validate ./bundle

.PHONY: validate-bundle
validate-bundle:
	$(OPERATOR_SDK_BIN) bundle validate $(BUNDLE_DIR) --select-optional suite=operatorframework

.PHONY: bundle-build
bundle-build: bundle
	$(CONTAINER_ENGINE) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) container-push IMG=$(BUNDLE_IMG)

.PHONY: index-image-build
index-image-build: opm
	$(OPM) index add -c $(CONTAINER_ENGINE) --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG}

.PHONY: index-image-push
index-image-push:
	$(CONTAINER_ENGINE) push ${INDEX_IMG}

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.19.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(BUNDLE_VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) container-push IMG=$(CATALOG_IMG)


.PHONY: lint
## Checks the code with golangci-lint
lint:
	$(GOLANGCI_LINT_BIN) run -c .golangci.yaml --deadline=30m
