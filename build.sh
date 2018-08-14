#!/bin/sh

# Variables can be overridden from command line, e.g.
# DOCKER_REGISTRY_USER=<my-username> ./build.sh
DOCKER_REGISTRY_NAME=${DOCKER_REGISTRY_NAME:-"quay.io"}
DOCKER_REGISTRY_USER=${DOCKER_REGISTRY_USER:-"kubernetes_incubator"}
DOCKER_IMAGE_NAME=${DOCKER_IMAGE_NAME:-"node-feature-discovery"}

DOCKER_TAG_BASE=$DOCKER_REGISTRY_NAME/$DOCKER_REGISTRY_USER/$DOCKER_IMAGE_NAME
VERSION=$(git describe --tags --dirty --always)

# Build NFD intermediate (builder) image
docker build --build-arg NFD_VERSION=$VERSION \
             --build-arg http_proxy=$http_proxy \
             --build-arg HTTP_PROXY=$HTTP_PROXY \
             --build-arg https_proxy=$https_proxy \
             --build-arg HTTPS_PROXY=$HTTPS_PROXY \
             --build-arg no_proxy=$no_proxy \
             --build-arg NO_PROXY=$NO_PROXY \
             -t $DOCKER_TAG_BASE:build . -f Dockerfile.build

# Copy artefacts from the builder image
TMPDIR=$(mktemp -d -p . tmp.docker-build.XXX)
docker container create --name tmp $DOCKER_TAG_BASE:build
docker container cp tmp:/go/bin/node-feature-discovery $TMPDIR
docker container cp tmp:/usr/local/bin $TMPDIR
docker container cp tmp:/usr/local/lib $TMPDIR
docker container rm -f tmp

# Build production image
docker build --build-arg TMPDIR=$TMPDIR \
             -t $DOCKER_TAG_BASE:$VERSION .

rm -rf $TMPDIR
