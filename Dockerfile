ARG BUILDER_IMAGE
ARG BASE_IMAGE_FULL
ARG BASE_IMAGE_MINIMAL

# Build node feature discovery
FROM ${BUILDER_IMAGE} AS builder

# Get (cache) deps in a separate layer
COPY go.mod go.sum /go/node-feature-discovery/
COPY api/nfd/go.mod api/nfd/go.sum /go/node-feature-discovery/api/nfd/

WORKDIR /go/node-feature-discovery

RUN --mount=type=cache,target=/go/pkg/mod/ \
    go mod download

# Do actual build
ARG VERSION
ARG HOSTMOUNT_PREFIX

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=src=.,target=. \
    make install VERSION=$VERSION HOSTMOUNT_PREFIX=$HOSTMOUNT_PREFIX

# Create full variant of the production image
FROM ${BASE_IMAGE_FULL} AS full

# Run as unprivileged user
USER 65534:65534

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY deployment/components/worker-config/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
COPY --from=builder /go/bin/* /usr/bin/

# Create minimal variant of the production image
FROM ${BASE_IMAGE_MINIMAL} AS minimal

# Run as unprivileged user
USER 65534:65534

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY deployment/components/worker-config/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
COPY --from=builder /go/bin/* /usr/bin/
