# Build node feature discovery
FROM golang:1.8 as builder

ADD . /go/src/sigs.k8s.io/node-feature-discovery

WORKDIR /go/src/sigs.k8s.io/node-feature-discovery

ARG NFD_VERSION

RUN go get github.com/Masterminds/glide
RUN glide install --strip-vendor
RUN go install \
  -ldflags "-s -w -X main.version=$NFD_VERSION" \
  sigs.k8s.io/node-feature-discovery
RUN install -D -m644 node-feature-discovery.conf.example /etc/kubernetes/node-feature-discovery/node-feature-discovery.conf

RUN go test .


# Create production image for running node feature discovery
FROM debian:stretch-slim

COPY --from=builder /etc/kubernetes/node-feature-discovery /etc/kubernetes/node-feature-discovery
COPY --from=builder /go/bin/node-feature-discovery /usr/bin/node-feature-discovery

ENTRYPOINT ["/usr/bin/node-feature-discovery"]
