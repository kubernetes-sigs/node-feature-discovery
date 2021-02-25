#!/bin/bash -e

# Install mdl
gem install mdl -v 0.11.0

# Run verify steps
make mdlint
