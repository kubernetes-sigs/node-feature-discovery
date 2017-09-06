# Taken from https://github.com/docker-library/golang/blob/master/1.6/Dockerfile 
# to build our golang image with debian testing (stretch).
FROM buildpack-deps:stretch-scm

# gcc for cgo.
RUN apt-get update && apt-get install -y --no-install-recommends \
		g++ \
		gcc \
		libc6-dev \
		make \
	&& rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.7.1
ENV GOLANG_DOWNLOAD_URL https://golang.org/dl/go$GOLANG_VERSION.linux-amd64.tar.gz
ENV GOLANG_DOWNLOAD_SHA256 43ad621c9b014cde8db17393dc108378d37bc853aa351a6c74bf6432c1bbd182

RUN curl -fsSL "$GOLANG_DOWNLOAD_URL" -o golang.tar.gz \
	&& echo "$GOLANG_DOWNLOAD_SHA256  golang.tar.gz" | sha256sum -c - \
	&& tar -C /usr/local -xzf golang.tar.gz \
	&& rm golang.tar.gz

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"
WORKDIR $GOPATH

# Build node feature discovery and set it as entrypoint.
ADD . /go/src/k8s.io/node-feature-discovery

WORKDIR /go/src/k8s.io/node-feature-discovery

ARG NFD_VERSION
RUN git clone --depth 1 https://github.com/01org/intel-cmt-cat.git
RUN cd intel-cmt-cat/lib; make install
RUN cd rdt-discovery; make
RUN go get github.com/tools/godep
RUN go install \
  -ldflags "-s -w -X main.version=$NFD_VERSION" \
  k8s.io/node-feature-discovery

ENTRYPOINT ["/go/bin/node-feature-discovery"]
