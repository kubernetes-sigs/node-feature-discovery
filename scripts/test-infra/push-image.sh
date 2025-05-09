#!/bin/bash -e

# Override VERSION if _GIT_TAG is specified. Strip 10 first characters
# ('vYYYYMMDD-') from _GIT_TAG in order to get a reproducible version and
# container image tag
VERSION_OVERRIDE=${_GIT_TAG+VERSION=${_GIT_TAG:10}}

# Authenticate in order to be able to push images
gcloud auth configure-docker

# Build and push images
IMAGE_ALL_PLATFORMS=linux/amd64,linux/arm64 make push-all $VERSION_OVERRIDE

go install helm.sh/helm/v3/cmd/helm@v3.17.3

make helm-push $VERSION_OVERRIDE
