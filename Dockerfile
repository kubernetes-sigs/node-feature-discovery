# Build node feature discovery
FROM golang:1.10 as builder

ADD . /go/src/sigs.k8s.io/node-feature-discovery

WORKDIR /go/src/sigs.k8s.io/node-feature-discovery

ARG NFD_VERSION

RUN go get github.com/golang/dep/cmd/dep
RUN dep ensure
RUN go install \
  -ldflags "-s -w -X sigs.k8s.io/node-feature-discovery/pkg/version.version=$NFD_VERSION" \
  ./cmd/*
RUN install -D -m644 nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf

#RUN go test .


# Create production image for running node feature discovery
FROM debian:stretch-slim

COPY --from=builder /etc/kubernetes/node-feature-discovery /etc/kubernetes/node-feature-discovery
COPY --from=builder /go/bin/nfd-* /usr/bin/
