#!/bin/bash -e

# local build
make image

# cross build
make image-all
