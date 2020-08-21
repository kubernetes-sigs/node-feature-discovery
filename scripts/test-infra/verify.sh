#!/bin/bash -e

# Install deps
curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(go env GOPATH)/bin v1.27.0

# Run verify steps
make gofmt-verify -e
make ci-lint -e
