# Build node feature discovery
#
# NOTE: we keep the builder stage of nfd-master and nfd-worker Dockerfiles in
# sync in order to re-use docker layers
#
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

# Create production image for nfd-master
FROM gcr.io/distroless/static

# Use more verbose logging of gRPC
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY --from=builder /go/bin/nfd-master /usr/bin/

ENTRYPOINT ["/usr/bin/nfd-master"]
