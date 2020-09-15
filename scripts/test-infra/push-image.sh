#!/bin/bash -e

# Override VERSION if _GIT_TAG is specified. Strip 10 first characters
# ('vYYYYMMDD-') from _GIT_TAG in order to get a reproducible version and
# container image tag
VERSION_OVERRIDE=${_GIT_TAG+VERSION=${_GIT_TAG:10}}

make image $VERSION_OVERRIDE
make push $VERSION_OVERRIDE
