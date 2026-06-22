#!/bin/bash -e

this_dir=`dirname $0`

# Tool versions
GOLANGCI_LINT_VERSION="v2.11.4"
HELM_VERSION="v3.17.3"
KUBECTL_VERSION="v1.22.1"

# Install deps
gobinpath="$(go env GOPATH)/bin"
curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b "$gobinpath" $GOLANGCI_LINT_VERSION
export PATH=$PATH:$(go env GOPATH)/bin

curl -sfL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash -s -- --version $HELM_VERSION

kubectl="$gobinpath/kubectl"
curl -L https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl -o "$kubectl"
chmod 755 "$kubectl"

# TODO: update logcheck version when there is a new release (newer than v0.9.0)
go install sigs.k8s.io/logtools/logcheck@v0.9.1-0.20251007102500-d35c84c015fe

# Run verify steps
echo "Checking gofmt"
make gofmt-verify

echo "Running golangci-lint"
make ci-lint

echo "Running Helm lint"
make helm-lint

echo "Running logcheck"
logcheck -config "${this_dir}/logcheck.conf" ./cmd/... ./pkg/...  ./source/...

echo "Running unit tests"
make test

# Upload coverage report (best-effort; coverage reporting must never gate CI).
# The legacy Codecov uploader is deprecated and its download/key endpoints have
# proven unreliable (e.g. the Keybase PGP key URL now returns a stub), so any
# failure here is logged and treated as non-fatal. Migrating to the supported
# Codecov CLI is tracked separately.
upload_coverage() {
    curl -Os https://uploader.codecov.io/latest/linux/codecov
    chmod +x codecov
    ./codecov -t "${CODECOV_TOKEN}" \
              -C "${PULL_PULL_SHA}" \
              -r "${REPO_OWNER}/${REPO_NAME}" \
              -P "${PULL_NUMBER}" \
              -b "${BUILD_ID}" \
              -B "${PULL_BASE_REF}" \
              -N "${PULL_BASE_SHA}"
}

echo "Uploading coverage report (best-effort, non-gating)"
if ! upload_coverage; then
    echo "WARNING: Codecov coverage upload failed; continuing because coverage reporting does not gate CI."
fi

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

# Check that the kustomize overlays are buildable
for d in `ls deployment/overlays/* -d`; do
    if [ "`basename $d`" = "samples" ]; then
        continue
    fi

    echo "Verifying $d"
    kubectl kustomize $d > /dev/null
done

# Check that the Helm validation schema is in sync
echo "Verifying Helm values schema"
make helm-schema
if ! git diff --quiet; then
    echo "Helm validation schema is not in sync. Run 'make helm-generate' to update"
    exit 1
fi

# Check that the Helm README is in sync
echo "Verifying Helm README"
make helm-docs
if ! git diff --quiet; then
    echo "Helm README is not in sync. Run 'make helm-generate' to update"
    exit 1
fi
