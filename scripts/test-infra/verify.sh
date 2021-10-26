#!/bin/bash -e

# Install deps
curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(go env GOPATH)/bin v1.36.0
export PATH=$PATH:$(go env GOPATH)/bin

curl -sfL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash -s -- --version v3.5.2

# Run verify steps
make ci-lint
make helm-lint

# Check that repo is clean
if ! git diff --quiet; then
    echo "Repository is dirty!"
    exit 1
fi

# Check that templates are up-to-date
make templates
if ! git diff --quiet; then
    echo "Deployment templates are not up-to-date. Run 'make templates' to update"
    exit 1
fi
