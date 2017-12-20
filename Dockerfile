FROM golang:1.8 

# Build node feature discovery and set it as entrypoint.
ADD . /go/src/github.com/kubernetes-incubator/node-feature-discovery

WORKDIR /go/src/github.com/kubernetes-incubator/node-feature-discovery

ARG NFD_VERSION

RUN case $(dpkg --print-architecture) in \
        arm64) \
                echo "skip rdt on Arm64 platform" \
                ;; \
        *) \
                git clone --depth 1 https://github.com/01org/intel-cmt-cat.git \
                && cd intel-cmt-cat/lib; make install \
                && cd ../../rdt-discovery; make \
                ;; \
        esac

RUN go get github.com/Masterminds/glide
RUN glide install --strip-vendor
RUN go install \
  -ldflags "-s -w -X main.version=$NFD_VERSION" \
  github.com/kubernetes-incubator/node-feature-discovery

ENTRYPOINT ["/go/bin/node-feature-discovery"]
