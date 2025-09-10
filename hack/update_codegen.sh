#!/usr/bin/env bash

# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

GO_CMD=${1:-go}
NFD_ROOT=$(realpath $(dirname ${BASH_SOURCE[0]})/..)

# Go generate
"${GO_CMD}" generate ./cmd/... ./pkg/... ./source/...

# Generate CRDs
go tool controller-gen object crd output:crd:stdout paths=./api/... > deployment/base/nfd-crds/nfd-api-crds.yaml
mkdir -p deployment/helm/node-feature-discovery/crds
cp deployment/base/nfd-crds/nfd-api-crds.yaml deployment/helm/node-feature-discovery/crds

# Generate clientset and informers
CODEGEN_PKG=$(go list -m -f '{{.Dir}}' k8s.io/code-generator)

cd $(dirname ${BASH_SOURCE[0]})/..

source "${CODEGEN_PKG}/kube_codegen.sh"

# Generating conversion and defaults functions
kube::codegen::gen_helpers \
  ${NFD_ROOT}/api/nfd \
  --boilerplate ${NFD_ROOT}/hack/boilerplate.go.txt

# Switch to work in the api worktree
pushd "${NFD_ROOT}/api/nfd" > /dev/null

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${NFD_ROOT}/api/generated" \
    --output-pkg "sigs.k8s.io/node-feature-discovery/api/generated" \
    --boilerplate "${NFD_ROOT}/hack/boilerplate.go.txt" \
    ${NFD_ROOT}/api

popd > /dev/null
