.PHONY: all

IMAGE_BUILD_CMD := docker build

QUAY_DOMAIN_NAME := quay.io
QUAY_REGISTRY_USER := kubernetes_incubator
DOCKER_IMAGE_NAME := node-feature-discovery

VERSION := $(shell git describe --tags --dirty --always)
ARCH := $(shell uname -m)
ifneq ($(ARCH), aarch64)
	ARCH_TOOLS = intel_cmt_cat
	ARCH_SUBDIRS = rdt_discovery
endif

all: image

intel_cmt_cat:
	$(MAKE) -C intel-cmt-cat/lib install

rdt_discovery:
	$(MAKE) -C rdt-discovery

install_tools: $(ARCH_TOOLS)
	@echo $(ARCH_TOOLS)

install: $(ARCH_SUBDIRS)
	glide install --strip-vendor
	go install \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		github.com/kubernetes-incubator/node-feature-discovery

# To override QUAY_REGISTRY_USER use the -e option as follows:
# QUAY_REGISTRY_USER=<my-username> make docker -e.
image:
	$(IMAGE_BUILD_CMD) \
		--build-arg http_proxy=$(http_proxy) \
		--build-arg HTTP_PROXY=$(HTTP_PROXY) \
		--build-arg https_proxy=$(https_proxy) \
		--build-arg HTTPS_PROXY=$(HTTPS_PROXY) \
		--build-arg no_proxy=$(no_proxy) \
		--build-arg NO_PROXY=$(NO_PROXY) \
		-t $(QUAY_DOMAIN_NAME)/$(QUAY_REGISTRY_USER)/$(DOCKER_IMAGE_NAME):$(VERSION) ./
