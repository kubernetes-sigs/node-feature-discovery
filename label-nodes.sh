#!/usr/bin/env bash
this=`basename $0`
if [ $# -gt 1 ]; then
    echo Usage: $this [IMAGE[:TAG]]
    exit 1
fi

# Get the number of nodes in Ready state in the Kubernetes cluster
NumNodes=$(kubectl get nodes | grep -i ' ready ' | wc -l)

# We set the .spec.completions and .spec.parallelism to the node count
# We request a specific hostPort in the job spec to limit the number of pods
# that run on a node to one. As a result, one pod runs on each node in parallel
# We set the NODE_NAME environemnt variable to get the Kubernetes node object.
sed -e "s/COMPLETION_COUNT/$NumNodes/" \
    -e "s/PARALLELISM_COUNT/$NumNodes/" \
    nfd-worker-job.yaml.template > nfd-worker-job.yaml

if [ -n "$1" ]; then
    sed -E "s,^(\s*)image:.+$,\1image: $1," -i nfd-worker-job.yaml
fi

kubectl create -f nfd-worker-job.yaml
