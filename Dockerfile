# Build node feature discovery
FROM golang:1.10 as builder

ADD . /go/src/sigs.k8s.io/node-feature-discovery

WORKDIR /go/src/sigs.k8s.io/node-feature-discovery

ENV CMT_CAT_VERSION="v1.2.0"

ARG NFD_VERSION

RUN case $(dpkg --print-architecture) in \
        arm64) \
                echo "skip rdt on Arm64 platform" \
                ;; \
        *) \
                git clone --depth 1 -b $CMT_CAT_VERSION https://github.com/intel/intel-cmt-cat.git && \
                make -C intel-cmt-cat/lib install && \
                make -C rdt-discovery && \
                make -C rdt-discovery install \
                ;; \
        esac

RUN go get github.com/golang/dep/cmd/dep
RUN dep ensure
RUN go install \
  -ldflags "-s -w -X main.version=$NFD_VERSION" \
  sigs.k8s.io/node-feature-discovery
RUN install -D -m644 node-feature-discovery.conf.example /etc/kubernetes/node-feature-discovery/node-feature-discovery.conf

RUN go test .


# Create production image for running node feature discovery
FROM debian:stretch-slim

COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /usr/local/lib /usr/local/lib
COPY --from=builder /etc/kubernetes/node-feature-discovery /etc/kubernetes/node-feature-discovery
RUN ldconfig
COPY --from=builder /go/bin/node-feature-discovery /usr/bin/node-feature-discovery

ENTRYPOINT ["/usr/bin/node-feature-discovery"]
