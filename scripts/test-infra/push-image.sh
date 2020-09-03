#!/bin/bash -e

# Strip 'vYYYYMMDD-' from the variable in order to get a reproducible
# version and container image tag
if [ -n "$_GIT_TAG" ]; then
    export VERSION=${_GIT_TAG:10}
fi

make image -e
make push -e
