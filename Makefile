.PHONY: all

QUAY_DOMAIN_NAME := quay.io
QUAY_REGISTRY_USER := kubernetes_incubator
DOCKER_IMAGE_NAME := node-feature-discovery

VERSION := $(shell git describe --tags --dirty --always)

DOCKER_TAG = $(QUAY_DOMAIN_NAME)/$(QUAY_REGISTRY_USER)/$(DOCKER_IMAGE_NAME):$(VERSION)

all: docker

# To override QUAY_REGISTRY_USER use the -e option as follows:
# QUAY_REGISTRY_USER=<my-username> make docker -e.
docker:
	docker build --build-arg NFD_VERSION=$(VERSION) \
		--build-arg http_proxy=$(http_proxy) \
		--build-arg HTTP_PROXY=$(HTTP_PROXY) \
		--build-arg https_proxy=$(https_proxy) \
		--build-arg HTTPS_PROXY=$(HTTPS_PROXY) \
		--build-arg no_proxy=$(no_proxy) \
		--build-arg NO_PROXY=$(NO_PROXY) \
		-t $(DOCKER_TAG) ./

	@echo "Copying nfdadm tool from container..."
	@container_id=`docker create $(DOCKER_TAG)`; \
	docker cp $${container_id}:/go/bin/nfdadm .; \
	docker rm -v $${container_id} > /dev/null

nfdadm:
	go install \
		-ldflags "-s -w -X github.com/kubernetes-incubator/node-feature-discovery/version.version=$(VERSION)" \
		github.com/kubernetes-incubator/node-feature-discovery/tools/nfdadm
