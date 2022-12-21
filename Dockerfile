ARG BUILDER_IMAGE
ARG BASE_IMAGE_FULL
ARG BASE_IMAGE_MINIMAL

# Build node feature discovery
FROM ${BUILDER_IMAGE} as builder

# Build and install the grpc-health-probe binary
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.14 && \
	go install github.com/grpc-ecosystem/grpc-health-probe@${GRPC_HEALTH_PROBE_VERSION} \
        # Rename it as it's referenced as grpc_health_probe in the deployment yamls
        # and in its own project https://github.com/grpc-ecosystem/grpc-health-probe
        && mv /go/bin/grpc-health-probe /go/bin/grpc_health_probe

# Get (cache) deps in a separate layer
COPY go.mod go.sum /go/node-feature-discovery/

WORKDIR /go/node-feature-discovery

RUN go mod download

# Do actual build
COPY . /go/node-feature-discovery

ARG VERSION
ARG HOSTMOUNT_PREFIX

RUN make install VERSION=$VERSION HOSTMOUNT_PREFIX=$HOSTMOUNT_PREFIX

RUN make test

# Create full variant of the production image
FROM ${BASE_IMAGE_FULL} as full

# Run as unprivileged user
USER 65534:65534

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY --from=builder /go/node-feature-discovery/deployment/components/worker-config/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
COPY --from=builder /go/bin/* /usr/bin/

# Create minimal variant of the production image
FROM ${BASE_IMAGE_MINIMAL} as minimal

# Run as unprivileged user
USER 65534:65534

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY --from=builder /go/node-feature-discovery/deployment/components/worker-config/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
COPY --from=builder /go/bin/* /usr/bin/
