#!/bin/bash -e

# Install mdl
gem install mdl -v 0.13.0
# Run verify steps
find docs/ -path docs/vendor -prune -false -o -name '*.md' -not -name 'generated*' | xargs mdl -s docs/mdl-style.rb
