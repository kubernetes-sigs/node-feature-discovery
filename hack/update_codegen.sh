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

go mod vendor

CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${NFD_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

cd $(dirname ${BASH_SOURCE[0]})/..

source "${CODEGEN_PKG}/kube_codegen.sh"

# TODO: remove the workaround when the issue is solved in the code-generator
# (https://github.com/kubernetes/code-generator/issues/165).
# Here, we create the soft link named "sigs.k8s.io" to the parent directory of
# node-feature-discovery to ensure the layout required by the kube_codegen.sh script.
ln -s .. sigs.k8s.io
trap "rm sigs.k8s.io" EXIT

CODEGEN_PKG=${CODEGEN_PKG:-$(ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

source "${CODEGEN_PKG}/kube_codegen.sh"

go generate ./cmd/... ./pkg/... ./source/...

controller-gen object crd output:crd:stdout paths=./pkg/apis/... > deployment/base/nfd-crds/nfd-api-crds.yaml

mkdir -p deployment/helm/node-feature-discovery/crds
cp deployment/base/nfd-crds/nfd-api-crds.yaml deployment/helm/node-feature-discovery/crds

# Generating conversion and defaults functions
kube::codegen::gen_helpers \
  --input-pkg-root sigs.k8s.io/node-feature-discovery/apis \
  --output-base "${NFD_ROOT}" \
  --boilerplate ${NFD_ROOT}/hack/boilerplate.go.txt

kube::codegen::gen_client \
  --input-pkg-root sigs.k8s.io/node-feature-discovery/pkg/apis \
  --output-pkg-root sigs.k8s.io/node-feature-discovery/pkg/generated \
  --output-base "${NFD_ROOT}" \
  --boilerplate ${NFD_ROOT}/hack/boilerplate.go.txt \
  --with-watch

# HACK: manually patching the auto-generated code as code-generator cannot
# properly handle deepcopy of MatchExpressionSet.
sed s'/out = new(map\[string\]\*MatchExpression)/out = new(MatchExpressionSet)/' -i pkg/apis/nfd/v1alpha1/zz_generated.deepcopy.go

# We need to clean up the go.mod file since code-generator adds temporary library to the go.mod file.
"${GO_CMD}" mod tidy