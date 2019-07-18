.PHONY: all test yamls
.FORCE:

IMAGE_BUILD_CMD := docker build
IMAGE_BUILD_EXTRA_OPTS :=
IMAGE_PUSH_CMD := docker push

VERSION := $(shell git describe --tags --dirty --always)

IMAGE_REGISTRY := quay.io/kubernetes_incubator
IMAGE_NAME := node-feature-discovery
IMAGE_TAG_NAME := $(VERSION)
IMAGE_REPO := $(IMAGE_REGISTRY)/$(IMAGE_NAME)
IMAGE_TAG := $(IMAGE_REPO):$(IMAGE_TAG_NAME)
K8S_NAMESPACE := kube-system
KUBECONFIG :=

yaml_templates := $(wildcard *.yaml.template)
yaml_instances := $(patsubst %.yaml.template,%.yaml,$(yaml_templates))

ARCH    ?= amd64
ALL_ARCH = amd64 arm64

IMAGEARCH ?=
QEMUARCH  ?=

ifneq ($(ARCH),amd64)
    IMAGE_TAG = $(IMAGE_REPO)-$(ARCH):$(IMAGE_TAG_NAME)
endif

ifeq ($(ARCH),amd64)
    IMAGEARCH =
    QEMUARCH  = x86_64
else ifeq ($(ARCH),arm)
    IMAGEARCH = arm32v7/
    QEMUARCH  = arm
else ifeq ($(ARCH),arm64)
    IMAGEARCH = arm64v8/
    QEMUARCH  = aarch64
else ifeq ($(ARCH),ppc64le)
    IMAGEARCH = ppc64le/
    QEMUARCH  = ppc64le
else ifeq ($(ARCH),s390x)
    IMAGEARCH = s390x/
    QEMUARCH  = s390x
else
    $(error unknown arch "$(ARCH)")
endif

all: image

image: yamls
	$(IMAGE_BUILD_CMD) --build-arg NFD_VERSION=$(VERSION) \
		--build-arg IMAGEARCH=$(IMAGEARCH) \
		--build-arg QEMUARCH=$(QEMUARCH) \
		-t $(IMAGE_TAG) \
		$(IMAGE_BUILD_EXTRA_OPTS) ./

pre-cross:
	docker run --rm --privileged multiarch/qemu-user-static:register --reset

# Building multi-arch docker images
images-all: pre-cross
	for arch in $(ALL_ARCH); do \
		$(MAKE) image ARCH=$$arch; \
	done

yamls: $(yaml_instances)

%.yaml: %.yaml.template .FORCE
	@echo "$@: namespace: ${K8S_NAMESPACE}"
	@echo "$@: image: ${IMAGE_TAG}"
	@sed -E \
	     -e s',^(\s*)name: node-feature-discovery # NFD namespace,\1name: ${K8S_NAMESPACE},' \
	     -e s',^(\s*)image:.+$$,\1image: ${IMAGE_TAG},' \
	     -e s',^(\s*)namespace:.+$$,\1namespace: ${K8S_NAMESPACE},' \
	     $< > $@

mock:
	mockery --name=FeatureSource --dir=source --inpkg --note="Re-generate by running 'make mock'"
	mockery --name=APIHelpers --dir=pkg/apihelper --inpkg --note="Re-generate by running 'make mock'"
	mockery --name=LabelerClient --dir=pkg/labeler --inpkg --note="Re-generate by running 'make mock'"

test:
	go test ./cmd/... ./pkg/...

e2e-test:
	dep ensure -v
	go test -v ./test/e2e/ -args -nfd.repo=$(IMAGE_REPO) -nfd.tag=$(IMAGE_TAG_NAME) -kubeconfig=$(KUBECONFIG)

push:
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)
