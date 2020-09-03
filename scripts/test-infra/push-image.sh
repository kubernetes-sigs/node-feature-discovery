#!/bin/bash -e

# Strip 'vYYYYMMDD-' from the variable in order to get a reproducible
# version and container image tag
if [ -n "$_GIT_TAG" ]; then
    export VERSION=${_GIT_TAG:10}
fi

# Authenticate in order to be able to push images
gcloud auth configure-docker

make image -e
make push -e
