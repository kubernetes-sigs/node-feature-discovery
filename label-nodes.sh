#!/usr/bin/env bash
# Get the node count in the Kubernetes cluster
NumNodes=$(kubectl get nodes | grep -i ready | wc -l)

# We set the .spec.completions and .spec.parallelism to the node count
# We request a specific hostPort in the job spec to limit the number of pods
# that run on a node to one. As a result, one pod runs on each node in parallel
# We set the POD_NAME and POD_NAMESPACE environemnt variables to the pod name 
# and pod namespace. These enivornment variables are used by the feature 
# discovery software to get the Kubernetes pod and node object. 
sed -e "s/COMPLETION_COUNT/$NumNodes/" -e "s/PARALLELISM_COUNT/$NumNodes/" dbi-iafeature-discovery-job.json.template > dbi-iafeature-discovery-job.json
kubectl create -f dbi-iafeature-discovery-job.json
