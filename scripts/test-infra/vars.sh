#!/usr/bin/env bash
set -xe

export IMAGE_REGISTRY=${IMAGE_REGISTRY:-"gcr.io/k8s-staging-nfd"}
export IMAGE_NAME=${IMAGE_NAME:-"node-feature-discovery"}
export IMAGE_TAG_NAME=${IMAGE_TAG_NAME:-"master"}
export KUBECONFIG=${KUBECONFIG:-"`pwd`/kubeconfig"}
export E2E_TEST_CONFIG=${E2E_TEST_CONFIG:-"`pwd`/e2e-test-config"}

