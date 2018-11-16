.PHONY: all test

IMAGE_BUILD_CMD := docker build

VERSION := $(shell git describe --tags --dirty --always)

IMAGE_REGISTRY := quay.io/kubernetes_incubator
IMAGE_NAME := node-feature-discovery
IMAGE_TAG_NAME := $(VERSION)
IMAGE_REPO := $(IMAGE_REGISTRY)/$(IMAGE_NAME)
IMAGE_TAG := $(IMAGE_REPO):$(IMAGE_TAG_NAME)


all: image

image:
	$(IMAGE_BUILD_CMD) --build-arg NFD_VERSION=$(VERSION) \
		-t $(IMAGE_TAG) ./

mock:
	mockery --name=FeatureSource --dir=source --inpkg --note="Re-generate by running 'make mock'"
	mockery --name=APIHelpers --dir=pkg/apihelper --inpkg --note="Re-generate by running 'make mock'"
	mockery --name=LabelerClient --dir=pkg/labeler --inpkg --note="Re-generate by running 'make mock'"

test:
	go test ./cmd/... ./pkg/...
