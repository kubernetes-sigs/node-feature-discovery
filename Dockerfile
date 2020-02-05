# Build node feature discovery
FROM golang:1.13.5 as builder

# Get (cache) deps in a separate layer
COPY go.mod go.sum /go/node-feature-discovery/

WORKDIR /go/node-feature-discovery

RUN go mod download

# Do actual build
COPY . /go/node-feature-discovery

ARG NFD_VERSION

RUN go install \
  -ldflags "-s -w -X sigs.k8s.io/node-feature-discovery/pkg/version.version=$NFD_VERSION" \
  ./cmd/*
RUN install -D -m644 nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf

RUN make test


# Create production image for running node feature discovery
FROM debian:stretch-slim

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY --from=builder /etc/kubernetes/node-feature-discovery /etc/kubernetes/node-feature-discovery
COPY --from=builder /go/bin/nfd-* /usr/bin/
