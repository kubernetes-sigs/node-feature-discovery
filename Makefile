.PHONY: all

DOCKER_REGISTRY_USER := intelsdi
DOCKER_IMAGE_NAME := nodelabels

VERSION := $(shell git describe --tags --dirty --always)

all: docker

# To override DOCKER_REGISTRY_USER use the -e option as follows:
# DOCKER_REGISTRY_USER=<my-username> make docker -e
docker:
	docker build -t $(DOCKER_REGISTRY_USER)/$(DOCKER_IMAGE_NAME):$(VERSION) ./
