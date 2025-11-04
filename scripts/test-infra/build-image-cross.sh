#!/bin/bash -e

# cross build
IMAGE_ALL_PLATFORMS=linux/amd64,linux/arm64,linux/s390x,linux/ppc64le make image-all
