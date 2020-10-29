#!/bin/bash -e

# Pre-create output directory with all write access. The Jekyll docker image is
# stupid enough to do all sorts of uid/gid/chown magic making build fail for
# root user. In prow we run as root because of DIND.
_outdir="docs/_site"
mkdir -p "$_outdir"
chmod a+rwx "$_outdir"

# Build docs
make site-build
