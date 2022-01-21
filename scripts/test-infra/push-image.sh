#!/bin/bash -e

# Override VERSION if _GIT_TAG is specified. Strip 10 first characters
# ('vYYYYMMDD-') from _GIT_TAG in order to get a reproducible version and
# container image tag
VERSION_OVERRIDE=${_GIT_TAG+VERSION=${_GIT_TAG:10}}

# Authenticate in order to be able to push images
gcloud auth configure-docker

# Build and push images
make push-all $VERSION_OVERRIDE
