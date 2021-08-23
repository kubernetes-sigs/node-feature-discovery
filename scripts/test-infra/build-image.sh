#!/bin/bash -e

echo "arm64"
make image IMAGE_BUILD_CMD="docker buildx build --platform linux/arm64"

echo "amd64"
make image IMAGE_BUILD_CMD="docker buildx build --platform linux/amd64"
