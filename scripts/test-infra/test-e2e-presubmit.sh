#!/bin/bash -e

set -o pipefail

export CLUSTER_NAME=$(git describe --tags --dirty --always)
export KUBECONFIG="/tmp/kubeconfig_$CLUSTER_NAME"

minikube start --bootstrapper=kubeadm \
  --vm-driver=docker \
  --memory 2048 \
  --cpus 2 \
  --profile $CLUSTER_NAME

eval $(minikube --profile $CLUSTER_NAME docker-env)

# Build the local container image
make image

# Use IfNotPresent image pull policy to use locally build image
export E2E_PULL_IF_NOT_PRESENT=true
make e2e-test || rc=$?

minikube delete --profile $CLUSTER_NAME

exit $rc
