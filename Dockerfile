FROM golang:1.8 

# Build node feature discovery and set it as entrypoint.
ADD . /go/src/github.com/kubernetes-incubator/node-feature-discovery

WORKDIR /go/src/github.com/kubernetes-incubator/node-feature-discovery

ENV PATH="/go/src/github.com/kubernetes-incubator/node-feature-discovery/rdt-discovery:${PATH}"

ENV CMT_CAT_VERSION="v1.2.0"

ARG NFD_VERSION

RUN case $(dpkg --print-architecture) in \
        arm64) \
                echo "skip rdt on Arm64 platform" \
                ;; \
        *) \
                git clone --depth 1 -b $CMT_CAT_VERSION https://github.com/01org/intel-cmt-cat.git && \
                make -C intel-cmt-cat/lib install && \
                make -C rdt-discovery \
                ;; \
        esac

RUN go get github.com/Masterminds/glide
RUN glide install --strip-vendor
RUN go install \
  -ldflags "-s -w -X github.com/kubernetes-incubator/node-feature-discovery/version.version=$NFD_VERSION" \
  github.com/kubernetes-incubator/node-feature-discovery

ENTRYPOINT ["/go/bin/node-feature-discovery"]
