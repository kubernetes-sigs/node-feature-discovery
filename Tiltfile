# -*- mode: Python -*-

BASE_IMAGE_MINIMAL="gcr.io/distroless/base"
BASE_IMAGE_FULL="debian:bullseye-slim"
BUILDER_IMAGE="golang:1.24rc3-bookworm"
HOSTMOUNT_PREFIX="/host-"
IMAGE_TAG_NAME = os.getenv('IMAGE_TAG_NAME', "master")
IMAGE_REGISTRY = os.getenv('IMAGE_REGISTRY', "gcr.io/k8s-staging-nfd")
IMAGE_NAME = os.getenv('IMAGE_NAME', "node-feature-discovery")

# Get the image name in the following format
# registry.k8s.io/nfd/node-feature-discovery:master
IMAGE = "/".join([IMAGE_REGISTRY, IMAGE_NAME])
TAGGED_IMAGE = ":".join([IMAGE, IMAGE_TAG_NAME])
allow_k8s_contexts('kubernetes-admin@kubernetes')

# Builds container image
def build_image():
    docker_build(
        TAGGED_IMAGE,
        context='.',
        build_args={
        "BUILDER_IMAGE": BUILDER_IMAGE,
        "BASE_IMAGE_MINIMAL": BASE_IMAGE_MINIMAL,
        "BASE_IMAGE_FULL": BASE_IMAGE_FULL,
        "HOSTMOUNT_PREFIX": HOSTMOUNT_PREFIX,
        },
        target="full",
        ignore=['./docs/', './examples/', './demo/']
    )

# Deploys manifests with kustomize
def deploy_nfd():
    k8s_yaml(
        kustomize('deployment/overlays/default/')
    )

# Actual calls to the functions
build_image()
deploy_nfd()
