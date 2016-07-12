#!/bin/bash

NumNodes=$(kubectl get nodes | grep -i ready | wc -l)
sed -e "s/COMPLETION_COUNT/$NumNodes/" -e "s/PARALLELISM_COUNT/$NumNodes/" featurelabeling-job.json.template > featurelabeling-job.json

kubectl create -f featurelabeling-job.json



