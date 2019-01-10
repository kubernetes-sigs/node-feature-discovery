# Build node feature discovery
FROM golang:1.10 as builder

ADD . /go/src/sigs.k8s.io/node-feature-discovery

WORKDIR /go/src/sigs.k8s.io/node-feature-discovery

ARG NFD_VERSION

RUN go get github.com/golang/dep/cmd/dep
RUN dep ensure
RUN go install \
  -ldflags "-s -w -X sigs.k8s.io/node-feature-discovery/pkg/version.version=$NFD_VERSION" \
  sigs.k8s.io/node-feature-discovery
RUN install -D -m644 node-feature-discovery.conf.example /etc/kubernetes/node-feature-discovery/node-feature-discovery.conf

RUN go test .


# Create production image for running node feature discovery
FROM debian:stretch-slim

COPY --from=builder /etc/kubernetes/node-feature-discovery /etc/kubernetes/node-feature-discovery
COPY --from=builder /go/bin/node-feature-discovery /usr/bin/node-feature-discovery

ENTRYPOINT ["/usr/bin/node-feature-discovery"]
