//go:build tools
// +build tools

package tools

import (
	// Adds targets for makefile
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/maxbrunsfeld/counterfeiter/v6"
	_ "github.com/openshift/build-machinery-go"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v4"
)
