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
FROM debian:testing

RUN apt-get update && apt-get install build-essential -y 
ADD http://parsec.cs.princeton.edu/download/3.0/parsec-3.0-core.tar.gz /root/
RUN cd /root; tar -xzf parsec-3.0-core.tar.gz
ADD https://s3.amazonaws.com/nfd-artifacts/parsec-ferret-input/input_native.tar /root/parsec-3.0/pkgs/apps/ferret/inputs/
WORKDIR /root/parsec-3.0
RUN ./bin/parsecmgmt -a build -p ferret -c gcc
ENTRYPOINT ["./bin/parsecmgmt", "-a", "run", "-p", "ferret", "-i", "native", "-n", "8"]
