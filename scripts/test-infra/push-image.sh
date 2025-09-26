#!/bin/bash -e

# Override VERSION if _GIT_TAG is specified. Strip 10 first characters
# ('vYYYYMMDD-') from _GIT_TAG in order to get a reproducible version and
# container image tag
if [ -n "$_GIT_TAG" ]; then
    MAKE_VARS="VERSION=${_GIT_TAG:10}"
fi
if ! [[ $_PULL_BASE_REF =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    MAKE_VARS="$MAKE_VARS IMAGE_EXTRA_TAG_NAMES=${_PULL_BASE_REF} CHART_EXTRA_VERSIONS=0.0.0-${_PULL_BASE_REF}"
fi

# Authenticate in order to be able to push images
gcloud auth configure-docker

# Build and push images
IMAGE_ALL_PLATFORMS=linux/amd64,linux/arm64 make push-all $MAKE_VARS

go install helm.sh/helm/v3/cmd/helm@v3.17.3
go install oras.land/oras/cmd/oras@v1.2.3

make helm-push $VERSION_OVERRIDE $MAKE_VARS
