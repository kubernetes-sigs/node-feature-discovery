.PHONY: all

QUAY_DOMAIN_NAME := quay.io
QUAY_REGISTRY_USER := kubernetes
DOCKER_IMAGE_NAME := node-feature-discovery

VERSION := $(shell git describe --tags --dirty --always)

all: docker

# To override QUAY_REGISTRY_USER use the -e option as follows:
# QUAY_REGISTRY_USER=<my-username> make docker -e.
docker:
	docker build --build-arg NFD_VERSION=$(VERSION) \
		-t $(QUAY_DOMAIN_NAME)/$(QUAY_REGISTRY_USER)/$(DOCKER_IMAGE_NAME):$(VERSION) ./
