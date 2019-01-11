#!/usr/bin/env bash
# Get the number of nodes in Ready state in the Kubernetes cluster
NumNodes=$(kubectl get nodes | grep -i ' ready ' | wc -l)

# We set the .spec.completions and .spec.parallelism to the node count
# We request a specific hostPort in the job spec to limit the number of pods
# that run on a node to one. As a result, one pod runs on each node in parallel
# We set the NODE_NAME environemnt variable to get the Kubernetes node object.
sed -e "s/COMPLETION_COUNT/$NumNodes/" -e "s/PARALLELISM_COUNT/$NumNodes/" nfd-worker-job.yaml.template > node-feature-discovery-job.yaml
kubectl create -f node-feature-discovery-job.yaml
