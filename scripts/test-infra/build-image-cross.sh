#!/bin/bash -e

# cross build
IMAGE_ALL_PLATFORMS=linux/amd64,linux/arm64 make image-all
