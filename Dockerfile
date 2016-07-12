#========================================================================
# Copyright 2016 Intel Corporation
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#    http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#========================================================================

FROM golang:latest

ADD ./main.go /go/src/github.com/intelsdi-x/dbi-iafeature-discovery/
ADD ./rdt-discovery/mon-discovery /go/bin/
ADD ./rdt-discovery/l3-alloc-discovery /go/bin/
ADD ./rdt-discovery/l2-alloc-discovery /go/bin/

WORKDIR /go/src/github.com/intelsdi-x/dbi-iafeature-discovery

RUN git clone https://github.com/01org/intel-cmt-cat.git
RUN cd intel-cmt-cat/lib; make install

RUN go get -d -v github.com/klauspost/cpuid
RUN go get -d -v k8s.io/kubernetes; exit 0

RUN go install github.com/intelsdi-x/dbi-iafeature-discovery

ENTRYPOINT /go/bin/dbi-iafeature-discovery
