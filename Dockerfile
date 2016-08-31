FROM golang:1.6

ADD . /go/src/github.com/kubernetes-incubator/node-feature-discovery

WORKDIR /go/src/github.com/kubernetes-incubator/node-feature-discovery

RUN git clone --depth 1 https://github.com/01org/intel-cmt-cat.git
RUN cd intel-cmt-cat/lib; make install
RUN cd rdt-discovery; make
RUN go get github.com/Masterminds/glide
RUN glide install
RUN go install \
  -ldflags "-s -w -X main.version=`git describe --tags --dirty --always`" \
  github.com/kubernetes-incubator/node-feature-discovery

ENTRYPOINT ["/go/bin/node-feature-discovery"]
