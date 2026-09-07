#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# One-shot check that every NFD release artifact for a given version is
# actually available: container images on registry.k8s.io, the OCI Helm
# chart, the gh-pages classic Helm repo index entry, and the GitHub release
# assets (chart .tgz and .tgz.prov). Written after v0.19.0 shipped with the
# container images promoted but the Helm chart digest missed, because the
# release checklist only mentioned images (see .github/ISSUE_TEMPLATE).

set -euo pipefail

this="$(basename "$0")"

usage() {
  cat <<EOF
Usage: $this VERSION

  VERSION  release version, with or without a leading "v" (e.g. 0.19.0 or v0.19.0)

Runs four checks for VERSION and exits non-zero if any of them fail:
  1. container images on registry.k8s.io (default, minimal, full)
  2. the OCI Helm chart on registry.k8s.io
  3. the gh-pages classic Helm repo index entry
  4. the GitHub release assets (chart .tgz and .tgz.prov)
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi

version="${1#v}"
tag="v${version}"

image_repo="registry.k8s.io/nfd/node-feature-discovery"
chart_oci_ref="oci://registry.k8s.io/nfd/charts/node-feature-discovery"
gh_pages_index_url="https://kubernetes-sigs.github.io/node-feature-discovery/charts/index.yaml"
gh_release_api_url="https://api.github.com/repos/kubernetes-sigs/node-feature-discovery/releases/tags/${tag}"

check_names=()
check_statuses=()
check_details=()

record() {
  check_names+=("$1")
  check_statuses+=("$2")
  check_details+=("${3:-}")
}

# resolve_manifest REF
# Resolves a container image manifest for REF (host/repo:tag), tolerating
# registry.k8s.io's 307 redirects: a strict HEAD probe false-negatives
# against it, a GET with proper Accept headers does not (learned 2026-07-10).
# Prefers crane, then docker (only when its daemon actually answers -- the
# docker CLI binary can be present with no daemon reachable, which would
# otherwise false-negative every image check), then falls back to a manual
# curl GET.
resolve_manifest() {
  local ref="$1"
  if command -v crane >/dev/null 2>&1; then
    crane digest "$ref" >/dev/null 2>&1
    return $?
  fi
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    docker manifest inspect "$ref" >/dev/null 2>&1
    return $?
  fi
  local host="${ref%%/*}"
  local rest="${ref#*/}"
  local repo_path="${rest%:*}"
  local manifest_tag="${rest##*:}"
  curl -sfL -o /dev/null \
    -H "Accept: application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json" \
    "https://${host}/v2/${repo_path}/manifests/${manifest_tag}"
}

check_images() {
  local suffix name ref
  for suffix in "" "-minimal" "-full"; do
    name="image ${tag}${suffix}"
    ref="${image_repo}:${tag}${suffix}"
    if resolve_manifest "$ref"; then
      record "$name" "PASS"
    else
      record "$name" "FAIL" "manifest not resolvable for ${ref}"
    fi
  done
}

check_oci_chart() {
  local name="OCI chart ${version}"
  local dir
  dir="$(mktemp -d)"
  if helm pull "$chart_oci_ref" --version "$version" -d "$dir" >/dev/null 2>&1; then
    record "$name" "PASS"
  else
    record "$name" "FAIL" "helm pull ${chart_oci_ref} --version ${version} did not resolve"
  fi
  rm -rf "$dir"
}

check_gh_pages_index() {
  local name="gh-pages Helm index ${version}"
  local index
  if ! index="$(curl -sfL "$gh_pages_index_url")"; then
    record "$name" "FAIL" "could not fetch ${gh_pages_index_url}"
    return
  fi
  local escaped_version="${version//./\\.}"
  if grep -qE "^[[:space:]]*version:[[:space:]]*${escaped_version}[[:space:]]*$" <<<"$index"; then
    record "$name" "PASS"
  else
    record "$name" "FAIL" "no entry with version: ${version} in ${gh_pages_index_url}"
  fi
}

check_gh_release_assets() {
  local name="GitHub release assets ${tag}"
  local json
  if ! json="$(curl -sfL "$gh_release_api_url")"; then
    record "$name" "FAIL" "could not fetch ${gh_release_api_url}"
    return
  fi
  local assets
  assets="$(jq -r '.assets[]?.name // empty' <<<"$json")"
  local chart_tgz="node-feature-discovery-chart-${version}.tgz"
  local chart_prov="${chart_tgz}.prov"
  local missing=()
  grep -qxF "$chart_tgz" <<<"$assets" || missing+=("$chart_tgz")
  grep -qxF "$chart_prov" <<<"$assets" || missing+=("$chart_prov")
  if [[ ${#missing[@]} -eq 0 ]]; then
    record "$name" "PASS"
  else
    record "$name" "FAIL" "missing asset(s): ${missing[*]}"
  fi
}

check_images
check_oci_chart
check_gh_pages_index
check_gh_release_assets

echo
echo "Release artifact verification for version ${version} (${tag})"
echo "-------------------------------------------------------------"
failed=0
for i in "${!check_names[@]}"; do
  status="${check_statuses[$i]}"
  detail="${check_details[$i]}"
  if [[ -n "$detail" ]]; then
    printf "%-30s %s %s\n" "${check_names[$i]}" "$status" "$detail"
  else
    printf "%-30s %s\n" "${check_names[$i]}" "$status"
  fi
  [[ "$status" == "FAIL" ]] && failed=1
done
echo "-------------------------------------------------------------"

if [[ "$failed" -eq 1 ]]; then
  echo "RESULT: FAIL - one or more release artifacts are missing for ${version}"
  exit 1
fi
echo "RESULT: PASS - all release artifacts present for ${version}"
