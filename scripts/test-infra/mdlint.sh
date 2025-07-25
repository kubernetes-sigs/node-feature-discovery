#!/bin/bash -e

# Install mdl
gem install mixlib-shellout -v 3.3.8
gem install mdl -v 0.11.0

# Run verify steps
make mdlint
