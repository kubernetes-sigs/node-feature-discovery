.PHONY: all test

IMAGE_BUILD_CMD := docker build

QUAY_DOMAIN_NAME := quay.io
QUAY_REGISTRY_USER := kubernetes_incubator
DOCKER_IMAGE_NAME := node-feature-discovery

VERSION := $(shell git describe --tags --dirty --always)

all: image

# To override QUAY_REGISTRY_USER use the -e option as follows:
# QUAY_REGISTRY_USER=<my-username> make docker -e.
image:
	$(IMAGE_BUILD_CMD) --build-arg NFD_VERSION=$(VERSION) \
		-t $(QUAY_DOMAIN_NAME)/$(QUAY_REGISTRY_USER)/$(DOCKER_IMAGE_NAME):$(VERSION) ./

mock:
	mockery --name=FeatureSource --dir=source --inpkg --note="Re-generate by running 'make mock'"
	mockery --name=APIHelpers --dir=pkg/apihelper --inpkg --note="Re-generate by running 'make mock'"
	mockery --name=LabelerClient --dir=pkg/labeler --inpkg --note="Re-generate by running 'make mock'"

test:
	go test ./cmd/... ./pkg/...
