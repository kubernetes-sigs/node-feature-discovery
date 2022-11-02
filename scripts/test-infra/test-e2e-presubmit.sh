#!/bin/bash -e
set -o pipefail

# Configure environment
KIND_IMAGE="kindest/node:v1.25.3"
export IMAGE_REGISTRY="localhost:5001"
export CLUSTER_NAME=$(git describe --tags --dirty --always)
export KUBECONFIG="/tmp/kubeconfig_$CLUSTER_NAME"

# create registry container unless it already exists
reg_name='kind-registry'
reg_port='5001'
if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
  docker run \
    -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" \
    registry:2
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --kubeconfig $KUBECONFIG --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $CLUSTER_NAME
nodes:
- role: control-plane
  image: $KIND_IMAGE
- role: worker
  image: $KIND_IMAGE
- role: worker
  image: $KIND_IMAGE
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:5000"]
EOF

# connect the registry to the cluster network if not already connected
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  docker network connect "kind" "${reg_name}"
fi

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

# build the local image
make image

# push the image to the local registry
make push

# run the tests
make e2e-test || rc=$?

# clean up the environment
kind delete cluster --kubeconfig $KUBECONFIG --name $CLUSTER_NAME
echo "Deleting ${reg_name} container ..."
docker rm -f "${reg_name}"
exit $rc
