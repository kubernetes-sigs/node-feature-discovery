FROM golang:1.8 

# Build node feature discovery and set it as entrypoint.
ADD . /go/src/github.com/kubernetes-incubator/node-feature-discovery

WORKDIR /go/src/github.com/kubernetes-incubator/node-feature-discovery

ENV PATH="/go/src/github.com/kubernetes-incubator/node-feature-discovery/rdt-discovery:${PATH}"

ARG NFD_VERSION

RUN case $(dpkg --print-architecture) in \
        arm64) \
                echo "skip rdt on Arm64 platform" \
                ;; \
        *) \
                make -C intel-cmt-cat/lib install && \
                make -C rdt-discovery \
                ;; \
        esac

RUN go get github.com/Masterminds/glide
RUN glide install --strip-vendor
RUN go install \
  -ldflags "-s -w -X main.version=$NFD_VERSION" \
  github.com/kubernetes-incubator/node-feature-discovery

ENTRYPOINT ["/go/bin/node-feature-discovery"]
