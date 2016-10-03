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

RUN apt-get update && apt-get install build-essential git gfortran -y 
RUN git clone --depth 1 https://github.com/UK-MAC/CloverLeaf_OpenMP.git /root/CloverLeaf_OpenMP
RUN cp /root/CloverLeaf_OpenMP/InputDecks/clover_bm_short.in /root/CloverLeaf_OpenMP/clover.in
RUN cd /root/CloverLeaf_OpenMP && make COMPILER=GNU MPI_COMPILER=gfortran C_MPI_COMPILER=gcc 
ENV OMP_NUM_THREADS 8
WORKDIR /root/CloverLeaf_OpenMP
ENTRYPOINT ["/bin/bash", "-c", "time ./clover_leaf"]
