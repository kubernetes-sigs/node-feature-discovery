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

TMP_VENDOR_DIR=gen-vendor

function cleanup() {
  echo "Cleaning up..."
  rm -rf ${TMP_VENDOR_DIR}
  rm sigs.k8s.io
  # We need to clean up the go.mod file since code-generator adds temporary library to the go.mod file.
  "${GO_CMD}" mod tidy
}

trap cleanup EXIT
GO_CMD=${1:-go}
NFD_ROOT=$(realpath $(dirname ${BASH_SOURCE[0]})/..)

"${GO_CMD}" mod vendor

# Go generate
"${GO_CMD}" generate ./cmd/... ./pkg/... ./source/...

# Generate CRDs
controller-gen object crd output:crd:stdout paths=./api/... > deployment/base/nfd-crds/nfd-api-crds.yaml
mkdir -p deployment/helm/node-feature-discovery/crds
cp deployment/base/nfd-crds/nfd-api-crds.yaml deployment/helm/node-feature-discovery/crds

# Generate clientset and informers
mv vendor ${TMP_VENDOR_DIR}
CODEGEN_PKG=${CODEGEN_PKG:-$(ls -d -1 ./${TMP_VENDOR_DIR}/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

cd $(dirname ${BASH_SOURCE[0]})/..

source "${CODEGEN_PKG}/kube_codegen.sh"

# TODO: https://github.com/kubernetes/code-generator/issues/165 has been closed
# we need to bump the version of code-generator to v0.30.0 once it's released
ln -s .. sigs.k8s.io

# Generating conversion and defaults functions
kube::codegen::gen_helpers \
  --input-pkg-root sigs.k8s.io/node-feature-discovery/api/nfd \
  --output-base "${NFD_ROOT}" \
  --boilerplate ${NFD_ROOT}/hack/boilerplate.go.txt

# HACK: manually patching the auto-generated code as code-generator cannot
# properly handle deepcopy of MatchExpressionSet.
sed s'/out = new(map\[string\]\*MatchExpression)/out = new(MatchExpressionSet)/' -i api/nfd/v1alpha1/zz_generated.deepcopy.go

kube::codegen::gen_client \
  --input-pkg-root sigs.k8s.io/node-feature-discovery/api \
  --output-pkg-root sigs.k8s.io/node-feature-discovery/api/generated \
  --output-base "${NFD_ROOT}" \
  --boilerplate ${NFD_ROOT}/hack/boilerplate.go.txt \
  --with-watch