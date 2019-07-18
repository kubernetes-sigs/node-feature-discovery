# Get qemu-user-static
ARG IMAGEARCH
FROM alpine:3.9.2 as qemu
RUN apk add --no-cache curl
ARG QEMUVERSION=2.9.1
ARG QEMUARCH

SHELL ["/bin/ash", "-o", "pipefail", "-c"]

RUN curl -fsSL https://github.com/multiarch/qemu-user-static/releases/download/v${QEMUVERSION}/qemu-${QEMUARCH}-static.tar.gz | tar zxvf - -C /usr/bin
RUN chmod +x /usr/bin/qemu-*

# Build node feature discovery
FROM ${IMAGEARCH}golang:1.12.7-stretch as builder
ENV GO_FLAGS=${GO_FLAGS:-"-tags netgo"}

ARG QEMUARCH
COPY --from=qemu /usr/bin/qemu-${QEMUARCH}-static /usr/bin/

ADD . /go/src/sigs.k8s.io/node-feature-discovery

WORKDIR /go/src/sigs.k8s.io/node-feature-discovery

ARG NFD_VERSION

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure -v
RUN go install \
  -ldflags "-s -w -X sigs.k8s.io/node-feature-discovery/pkg/version.version=$NFD_VERSION" \
  ./cmd/*
RUN install -D -m644 nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf

RUN make test

# Create production image for running node feature discovery
FROM ${IMAGEARCH}debian:stretch-slim

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY --from=builder /etc/kubernetes/node-feature-discovery /etc/kubernetes/node-feature-discovery
COPY --from=builder /go/bin/nfd-* /usr/bin/
