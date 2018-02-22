FROM golang:1.8 

# Build node feature discovery and set it as entrypoint.
ADD . /go/src/github.com/kubernetes-incubator/node-feature-discovery

WORKDIR /go/src/github.com/kubernetes-incubator/node-feature-discovery

ENV PATH="/go/src/github.com/kubernetes-incubator/node-feature-discovery/rdt-discovery:${PATH}"

RUN go get github.com/Masterminds/glide

RUN make install_tools && make install

ENTRYPOINT ["/go/bin/node-feature-discovery"]
