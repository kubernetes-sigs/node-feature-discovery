#!/bin/bash -e
set -o pipefail

this=`basename $0`

usage () {
cat << EOF
Usage: $this [-h] RELEASE_VERSION GPG_KEY GPG_KEYRING

Options:
  -h         show this help and exit

Example:

  $this v0.1.2 "Jane Doe <jane.doe@example.com>" ~/.gnupg/secring.gpg


NOTE: The GPG key should be associated with the signer's Github account.

NOTE: Helm is not compatible with GnuPG v2 and you need to export the secret
      keys in order for Helm to be able to sign the package:

  gpg --export-secret-keys > ~/.gnupg/secring.gpg

EOF
}

#
# Parse command line
#
while getopts "h" opt; do
    case $opt in
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
if [ $# -ne 3 ]; then
    if [ $# -lt 3 ]; then
        echo -e "ERROR: too few arguments\n"
    else
        echo -e "ERROR: unknown arguments: ${@:4}\n"
    fi
    usage
    exit 1
fi

release=$1
key="$2"
keyring="$3"
shift 3

container_image=k8s.gcr.io/nfd/node-feature-discovery:$release

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

# Patch docs configuration
echo Patching docs/_config.yml
sed -e s"/release:.*/release: $release/"  \
    -e s"/version:.*/version: $docs_version/" \
    -e s"!container_image:.*!container_image: k8s.gcr.io/nfd/node-feature-discovery:$release!" \
    -i docs/_config.yml

# Patch README
echo Patching README.md to refer to $release
sed s"!node-feature-discovery/v.*/!node-feature-discovery/$release/!" -i README.md

# Patch deployment templates
echo Patching '*.yaml.template' to use $container_image
sed -E -e s",^([[:space:]]+)image:.+$,\1image: $container_image," \
       -e s",^([[:space:]]+)imagePullPolicy:.+$,\1imagePullPolicy: IfNotPresent," \
       -i *yaml.template

# Patch Helm chart
sed -e s"/appVersion:.*/appVersion: $release/" -i deployment/node-feature-discovery/Chart.yaml
sed -e s"/pullPolicy:.*/pullPolicy: IfNotPresent/" \
    -e s"!gcr.io/k8s-staging-nfd/node-feature-discovery!k8s.gcr.io/nfd/node-feature-discovery!" \
    -i deployment/node-feature-discovery/values.yaml

# Patch e2e test
echo Patching test/e2e/node_feature_discovery.go flag defaults to k8s.gcr.io/nfd/node-feature-discovery and $release
sed -e s'!"nfd\.repo",.*,!"nfd.repo", "k8s.gcr.io/nfd/node-feature-discovery",!' \
    -e s"!\"nfd\.tag\",.*,!\"nfd.tag\", \"$release\",!" \
  -i test/e2e/node_feature_discovery.go

#
# Create release assets to be uploaded
#
helm package deployment/node-feature-discovery/ --version $semver --sign \
    --key "$key" --keyring "$keyring"

chart_name="node-feature-discovery-chart-$semver.tgz"
mv node-feature-discovery-$semver.tgz $chart_name
mv node-feature-discovery-$semver.tgz.prov $chart_name.prov

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
