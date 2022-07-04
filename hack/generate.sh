#!/bin/bash -e
set -o pipefail

# Default path for code-generator repo
K8S_CODE_GENERATOR=${K8S_CODE_GENERATOR:-../code-generator}

go mod vendor

go generate ./cmd/... ./pkg/... ./source/...

rm -rf vendor/

controller-gen object crd output:crd:stdout paths=./pkg/apis/... > deployment/base/nfd-crds/nfd-api-crds.yaml

mkdir -p deployment/helm/node-feature-discovery/crds
cp deployment/base/nfd-crds/nfd-api-crds.yaml deployment/helm/node-feature-discovery/crds

rm -rf sigs.k8s.io

${K8S_CODE_GENERATOR}/generate-groups.sh client,informer,lister \
    sigs.k8s.io/node-feature-discovery/pkg/generated \
    sigs.k8s.io/node-feature-discovery/pkg/apis \
    "nfd:v1alpha1" --output-base=. \
    --go-header-file hack/boilerplate.go.txt

rm -rf pkg/generated

mv sigs.k8s.io/node-feature-discovery/pkg/generated pkg/

rm -rf sigs.k8s.io

