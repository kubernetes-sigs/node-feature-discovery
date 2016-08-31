.PHONY: all

DOCKER_REGISTRY_USER := kubernetesincubator
DOCKER_IMAGE_NAME := node-feature-discovery

VERSION := $(shell git describe --tags --dirty --always)

all: docker

# To override DOCKER_REGISTRY_USER use the -e option as follows:
# DOCKER_REGISTRY_USER=<my-username> make docker -e
docker:
	docker build -t $(DOCKER_REGISTRY_USER)/$(DOCKER_IMAGE_NAME):$(VERSION) ./
