# Build node feature discovery
FROM golang:1.15.5-buster as builder

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


# Use debian-base for the extra files we need
FROM k8s.gcr.io/build-image/debian-base:v2.1.3 as extra

RUN apt-get update -y && apt-get -q -yy install --no-install-recommends --no-install-suggests --fix-missing busybox-static


# Create production image for running node feature discovery
FROM gcr.io/distroless/static

# Run as unprivileged user
USER 65534:65534

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY --from=builder /go/node-feature-discovery/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
COPY --from=builder /go/bin/* /usr/bin/
COPY --from=extra /bin/busybox /bin/sh
