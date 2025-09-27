#!/bin/bash -e
set -o pipefail

this=`basename $0`

usage () {
cat << EOF
Usage: $this [-h] [-b] [-k GPG_KEY] {-a|-g GOLANG_VERSION} RELEASE_VERSION

Options:
  -h         show this help and exit
  -a         do not patch files in the repo
  -b         do not generate assets
  -g         golang version to fix for the release (mandatory when -a not
             specified). Should be a exact point release e.g. 1.18.3.
  -k         gpg key to use for signing the assets

Example:

  $this -k "Jane Doe <jane.doe@example.com>" v0.13.1


NOTE: The GPG key should be associated with the signer's Github account.
      Use -k to specify the correct key (if needed).
EOF
}

sign_helm_chart() {
  local chart="$1"
  echo "Signing Helm chart $chart"
  local sha256=`openssl dgst -sha256 "$chart" | awk '{ print $2 }'`
  local yaml=`tar xf $chart -O node-feature-discovery/Chart.yaml`
  echo "$yaml
...
files:
  $chart: sha256:$sha256" | gpg ${signing_key:+-u "$signing_key"} --clearsign -o "$chart.prov"
}

#
# Parse command line
#
no_patching=
no_assets=
while getopts "abg:k:h" opt; do
    case $opt in
        a)  no_patching=y
            ;;
        b)  no_assets=y
            ;;
        g) golang_version="$OPTARG"
            ;;
        k) signing_key="$OPTARG"
            ;;
        h)  usage
            exit 0
            ;;
        *)  usage
            exit 1
            ;;
    esac
done
shift "$((OPTIND - 1))"

# Check that no extra args were provided
if [ $# -ne 1 ]; then
    if [ $# -lt 1 ]; then
        echo -e "ERROR: too few arguments\n"
    else
        echo -e "ERROR: unknown arguments: ${@:3}\n"
    fi
    usage
    exit 1
fi

if [ -z "$no_patching" -a -z "$golang_version" ]; then
    echo -e "ERROR: '-g GOLANG_VERSION' must be specified when modifying repo (i.e. when '-a' is not used)\n"
    usage
    exit 1
fi

release=$1
shift 1

container_image=registry.k8s.io/nfd/node-feature-discovery:$release

#
# Check/parse release number
#
if [ -z "$release" ]; then
    echo -e "ERROR: missing RELEASE_VERSION\n"
    usage
    exit 1
fi

if [[ $release =~ ^(v[0-9]+\.[0-9]+)(\..+)?$ ]]; then
    docs_version=${BASH_REMATCH[1]}
    semver=${release:1}
else
    echo -e "ERROR: invalid RELEASE_VERSION '$release'"
    exit 1
fi

#
# Modify files in the repo to point to new release
#
if [ -z "$no_patching" ]; then
    # Patch docs configuration
    echo Patching golang version $golang_version into Makefile
    sed -e s"/\(^BUILDER_IMAGE.*=.*golang:\)[0-9][0-9.]*\(.*\)/\1$golang_version\2/" \
        -i Makefile

    # Patch docs configuration
    echo Patching docs/_config.yml
    sed -e s"/release:.*/release: $release/"  \
        -e s"/version:.*/version: $docs_version/" \
        -e s"/helm_chart_version:.*/helm_chart_version: $semver/" \
        -e s"!container_image:.*!container_image: registry.k8s.io/nfd/node-feature-discovery:$release!" \
        -e s"!helm_oci_repo:.*!helm_oci_repo: oci://registry.k8s.io/nfd/charts/node-feature-discovery!" \
        -i docs/_config.yml

    # Patch README
    echo Patching README.md to refer to $release
    sed -e s"!\(node-feature-discovery/deployment/.*\)=v.*!\1=$release!" \
        -e s"!^\[documentation\]:.*![documentation]: https://kubernetes-sigs.github.io/node-feature-discovery/$docs_version!" \
        -i README.md

    # Patch deployment templates
    echo Patching kustomize templates to use $container_image
    find deployment/base deployment/overlays deployment/components -name '*.yaml' | xargs -I '{}' \
    sed -E -e s",^([[:space:]]+)image:.+$,\1image: $container_image," \
           -e s",^([[:space:]]+)imagePullPolicy:.+$,\1imagePullPolicy: IfNotPresent," \
           -i '{}'

    # Patch Helm chart
    echo "Patching Helm chart"
    sed -e s"/appVersion:.*/appVersion: $release/" \
        -e s"!icon:.*!icon: https://kubernetes-sigs.github.io/node-feature-discovery/$docs_version/assets/images/nfd/favicon.svg!" \
        -i deployment/helm/node-feature-discovery/Chart.yaml
    sed -e s"/pullPolicy:.*/pullPolicy: IfNotPresent/" \
        -e s"!gcr.io/k8s-staging-nfd/node-feature-discovery!registry.k8s.io/nfd/node-feature-discovery!" \
        -i deployment/helm/node-feature-discovery/values.yaml
    sed -e s"!kubernetes-sigs.github.io/node-feature-discovery/master!kubernetes-sigs.github.io/node-feature-discovery/$docs_version!" \
        -i deployment/helm/node-feature-discovery/README.md

    # Patch e2e test
    echo Patching test/e2e/node_feature_discovery.go flag defaults to registry.k8s.io/nfd/node-feature-discovery and $release
    sed -e s'!"nfd\.repo",.*,!"nfd.repo", "registry.k8s.io/nfd/node-feature-discovery",!' \
        -e s"!\"nfd\.tag\",.*,!\"nfd.tag\", \"$release\",!" \
      -i test/e2e/node_feature_discovery_test.go
fi

#
# Create release assets to be uploaded
#
if [ -z "$no_assets" ]; then
    helm package deployment/helm/node-feature-discovery/ --version $semver

    chart_name="node-feature-discovery-chart-$semver.tgz"
    mv node-feature-discovery-$semver.tgz $chart_name
    sign_helm_chart $chart_name

    cat << EOF

*******************************************************************************
*** Please manually upload the following generated files to the Github release
*** page:
***
***   $chart_name
***   $chart_name.prov
***
*******************************************************************************
EOF
fi
