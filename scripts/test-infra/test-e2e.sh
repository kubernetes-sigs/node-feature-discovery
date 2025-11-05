#!/bin/bash -e

# Configure environment
export KIND_VERSION="v0.23.0"
export KIND_NODE_IMAGE="kindest/node:v1.30.2"
export CLUSTER_NAME="nfd-e2e"
export KUBECONFIG=`pwd`/kubeconfig
export IMAGE_REGISTRY="gcr.io/k8s-staging-nfd"
export E2E_TEST_FULL_IMAGE=true

# Install kind
go install sigs.k8s.io/kind@$KIND_VERSION

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --kubeconfig $KUBECONFIG --image $KIND_NODE_IMAGE --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $CLUSTER_NAME
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

# Wait for the image to be built and published
i=1
while true; do
    if make poll-images; then
        break
    elif [ $i -ge 90 ]; then
        echo "ERROR: too many tries when polling for image"
        exit 1
    fi
    sleep 60

    i=$(( $i + 1 ))
done

# Configure environment and run tests
make e2e-test
