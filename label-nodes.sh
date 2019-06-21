#!/usr/bin/env bash
this=`basename $0`
if [ $# -gt 1 ] || [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
    echo Usage: $this [IMAGE[:TAG]]
    exit 1
fi

IMAGE=$1
if [ -n "$IMAGE" ]; then
    if [ ! -f nfd-worker-job.yaml ]; then
        make IMAGE_TAG=$IMAGE nfd-worker-job.yaml
    else
        # Keep existing nfd-worker-job.yaml, only update image.
        sed -E "s,^(\s*)image:.+$,\1image: $IMAGE," -i nfd-worker-job.yaml
    fi
fi

if [ ! -f nfd-worker-job.yaml ]; then
    # Missing image info for the labeling job.
    echo "nfd-worker-job.yaml missing."
    echo "Run 'make nfd-worker-job.yaml', use the template or provide IMAGE (see --help)."
    exit 2
fi

# Get the number of nodes in Ready state in the Kubernetes cluster
NumNodes=$(kubectl get nodes | grep -i ' ready ' | wc -l)

# We set the .spec.completions and .spec.parallelism to the node count
# We set the NODE_NAME environment variable to get the Kubernetes node object.
sed -e "s/completions:.*$/completions: $NumNodes/" \
    -e "s/parallelism:.*$/parallelism: $NumNodes/" \
    -i nfd-worker-job.yaml

kubectl create -f nfd-worker-job.yaml
