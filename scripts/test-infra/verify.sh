#!/bin/bash -e

this_dir=`dirname $0`

# Tool versions
GOLANGCI_LINT_VERSION="v2.14.0"
# SHA-256 of golangci-lint-<version>-linux-amd64.tar.gz, from the release's
# checksums file. Update it together with GOLANGCI_LINT_VERSION.
GOLANGCI_LINT_SHA256="ab90aeb7b066f92a33415b638a50fe5344bbb75a0d32ad30cc248d88f81032ab"
# The helm-unittest plugin installed by 'make helm-unittest' needs Helm v3.18.0
# or later: its plugin.yaml uses the platformHooks field, which older Helm
# versions reject.
HELM_VERSION="v3.22.1"
KUBECTL_VERSION="v1.22.1"
CODECOV_CLI_VERSION="v11.3.1"
CODECOV_CLI_URL="https://cli.codecov.io/${CODECOV_CLI_VERSION}/linux"
CODECOV_PGP_KEY_URL="https://keybase.io/codecovsecops/pgp_keys.asc"
CODECOV_PGP_FINGERPRINT="27034E7FDB850E0BBC2C62FF806BB28AED779869"

# Install deps
gobinpath="$(go env GOPATH)/bin"
mkdir -p "$gobinpath"

# golangci-lint's install.sh cannot install v2.13.0 or later: those checksums
# files also list a .sbom.json per tarball, and its unanchored grep picks up
# both lines. Fetch the tarball directly and check it against the pinned hash.
gcl_name="golangci-lint-${GOLANGCI_LINT_VERSION#v}-linux-amd64"
gcl_tmp="$(mktemp -d)"
curl -sSfL -o "$gcl_tmp/$gcl_name.tar.gz" "https://github.com/golangci/golangci-lint/releases/download/$GOLANGCI_LINT_VERSION/$gcl_name.tar.gz"
echo "$GOLANGCI_LINT_SHA256  $gcl_tmp/$gcl_name.tar.gz" | sha256sum -c -
tar -xzf "$gcl_tmp/$gcl_name.tar.gz" -C "$gobinpath" --strip-components=1 "$gcl_name/golangci-lint"
rm -rf "$gcl_tmp"
export PATH=$PATH:$(go env GOPATH)/bin

curl -sfL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash -s -- --version $HELM_VERSION

kubectl="$gobinpath/kubectl"
curl -L https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl -o "$kubectl"
chmod 755 "$kubectl"

go install sigs.k8s.io/logtools/logcheck@v0.10.1

# Run verify steps
echo "Checking gofmt"
make gofmt-verify

echo "Running golangci-lint"
make ci-lint

echo "Running Helm lint"
make helm-lint

echo "Running Helm unit tests"
make helm-unittest

echo "Running logcheck"
logcheck -config "${this_dir}/logcheck.conf" ./cmd/... ./pkg/...  ./source/...

echo "Running unit tests"
make test

# Upload coverage report (best-effort; coverage reporting must never gate CI).
upload_coverage() (
    local codecov_dir gpg_home fingerprint repo_dir

    repo_dir="$(pwd)"
    codecov_dir="$(mktemp -d)" || exit 1
    trap 'rm -rf "${codecov_dir}"' EXIT
    gpg_home="${codecov_dir}/gnupg"
    mkdir -m 700 "${gpg_home}" || exit 1

    curl -fsSL "${CODECOV_PGP_KEY_URL}" -o "${codecov_dir}/pgp_keys.asc" || exit 1
    gpg --batch --homedir "${gpg_home}" --import "${codecov_dir}/pgp_keys.asc" || exit 1

    fingerprint="$(gpg --batch --homedir "${gpg_home}" --with-colons \
        --list-keys "${CODECOV_PGP_FINGERPRINT}" | awk -F: '$1 == "fpr" { print $10; exit }')"
    if [ "${fingerprint}" != "${CODECOV_PGP_FINGERPRINT}" ]; then
        echo "Codecov signing key fingerprint did not match the expected key" >&2
        exit 1
    fi

    curl -fsSL "${CODECOV_CLI_URL}/codecov" -o "${codecov_dir}/codecov" || exit 1
    curl -fsSL "${CODECOV_CLI_URL}/codecov.SHA256SUM" -o "${codecov_dir}/codecov.SHA256SUM" || exit 1
    curl -fsSL "${CODECOV_CLI_URL}/codecov.SHA256SUM.sig" -o "${codecov_dir}/codecov.SHA256SUM.sig" || exit 1
    gpg --batch --homedir "${gpg_home}" --status-fd 1 --verify \
        "${codecov_dir}/codecov.SHA256SUM.sig" "${codecov_dir}/codecov.SHA256SUM" \
        | grep -q "^\[GNUPG:\] VALIDSIG .* ${CODECOV_PGP_FINGERPRINT}\$" || exit 1
    (cd "${codecov_dir}" && sha256sum --check codecov.SHA256SUM) || exit 1
    chmod +x "${codecov_dir}/codecov" || exit 1

    "${codecov_dir}/codecov" upload-process \
        --fail-on-error \
        --token "${CODECOV_TOKEN}" \
        --commit-sha "${PULL_PULL_SHA}" \
        --slug "${REPO_OWNER}/${REPO_NAME}" \
        --pull-request-number "${PULL_NUMBER}" \
        --build "${BUILD_ID}" \
        --branch "${PULL_BASE_REF}" \
        --parent-sha "${PULL_BASE_SHA}" \
        --git-service github \
        --dir "${repo_dir}"
)

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
