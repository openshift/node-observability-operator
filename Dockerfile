# Build the manager binary
FROM docker.io/golang:1.17 as builder
# TODO: Switch to UBI golang image when 1.17 is released
# FROM registry.access.redhat.com/ubi8/go-toolset:latest

WORKDIR /workspace

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY vendor/ vendor/
COPY go.mod go.mod
COPY go.sum go.sum
COPY Makefile Makefile

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build-operator

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi8/ubi-micro:8.5
WORKDIR /
COPY --from=builder /workspace/bin/node-observability-operator .
USER 65532:65532

ENTRYPOINT ["/node-observability-operator"]
