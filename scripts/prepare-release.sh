#!/bin/bash -e
set -o pipefail

this=`basename $0`

usage () {
cat << EOF
Usage: $this [-h] RELEASE_VERSION

Options:
  -h         show this help and exit
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
if [ $# -gt 1 ]; then
    echo -e "ERROR: unknown arguments: $@\n"
    usage
    exit 1
fi

release=$1
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
sed -E s",^([[:space:]]+)image:.+$,\1image: $container_image," -i *yaml.template

# Patch e2e test
echo Patching test/e2e/node_feature_discovery.go flag defaults to k8s.gcr.io/nfd/node-feature-discovery and $release
sed -e s'!"nfd\.repo",.*,!"nfd.repo", "k8s.gcr.io/nfd/node-feature-discovery",!' \
    -e s"!\"nfd\.tag\",.*,!\"nfd.tag\", \"$release\",!" \
  -i test/e2e/node_feature_discovery.go
