.PHONY: all test templates yamls
.FORCE:

GO_CMD ?= go
GO_FMT ?= gofmt

IMAGE_BUILD_CMD ?= docker build
IMAGE_BUILD_EXTRA_OPTS ?=
IMAGE_PUSH_CMD ?= docker push
CONTAINER_RUN_CMD ?= docker run
BASE_IMAGE_FULL ?= debian:bullseye-slim
BASE_IMAGE_MINIMAL ?= gcr.io/distroless/base

MDL ?= mdl

K8S_CODE_GENERATOR ?= ../code-generator

# Docker base command for working with html documentation.
# Use host networking because 'jekyll serve' is stupid enough to use the
# same site url than the "host" it binds to. Thus, all the links will be
# broken if we'd bind to 0.0.0.0
JEKYLL_VERSION := 3.8
JEKYLL_ENV ?= development
SITE_BUILD_CMD := $(CONTAINER_RUN_CMD) --rm -i -u "`id -u`:`id -g`" \
	-e JEKYLL_ENV=$(JEKYLL_ENV) \
	--volume="$$PWD/docs:/srv/jekyll" \
	--volume="$$PWD/docs/vendor/bundle:/usr/local/bundle" \
	--network=host jekyll/jekyll:$(JEKYLL_VERSION)
SITE_BASEURL ?=
SITE_DESTDIR ?= _site
JEKYLL_OPTS := -d '$(SITE_DESTDIR)' $(if $(SITE_BASEURL),-b '$(SITE_BASEURL)',)

VERSION := $(shell git describe --tags --dirty --always)

IMAGE_REGISTRY ?= k8s.gcr.io/nfd
IMAGE_TAG_NAME ?= $(VERSION)
IMAGE_EXTRA_TAG_NAMES ?=

IMAGE_NAME := node-feature-discovery
IMAGE_REPO := $(IMAGE_REGISTRY)/$(IMAGE_NAME)
IMAGE_TAG := $(IMAGE_REPO):$(IMAGE_TAG_NAME)
IMAGE_EXTRA_TAGS := $(foreach tag,$(IMAGE_EXTRA_TAG_NAMES),$(IMAGE_REPO):$(tag))

K8S_NAMESPACE ?= node-feature-discovery

OPENSHIFT ?=

# We use different mount prefix for local and container builds.
# Take CONTAINER_HOSTMOUNT_PREFIX from HOSTMOUNT_PREFIX if only the latter is specified
ifdef HOSTMOUNT_PREFIX
    CONTAINER_HOSTMOUNT_PREFIX := $(HOSTMOUNT_PREFIX)
else
    CONTAINER_HOSTMOUNT_PREFIX := /host-
endif
HOSTMOUNT_PREFIX ?= /

KUBECONFIG ?=
E2E_TEST_CONFIG ?=

LDFLAGS = -ldflags "-s -w -X sigs.k8s.io/node-feature-discovery/pkg/version.version=$(VERSION) -X sigs.k8s.io/node-feature-discovery/source.pathPrefix=$(HOSTMOUNT_PREFIX)"

# multi-arch build with buildx
IMAGE_ALL_PLATFORMS ?= linux/amd64,linux/arm64

# enable buildx
ensure-buildx:
	./hack/init-buildx.sh

IMAGE_BUILDX_CMD ?= DOCKER_CLI_EXPERIMENTAL=enabled docker buildx build --platform=${IMAGE_ALL_PLATFORMS} --progress=auto --pull

IMAGE_BUILD_ARGS = --build-arg VERSION=$(VERSION) \
	    	   --build-arg HOSTMOUNT_PREFIX=$(CONTAINER_HOSTMOUNT_PREFIX) \
	    	   --build-arg BASE_IMAGE_FULL=$(BASE_IMAGE_FULL) \
	    	   --build-arg BASE_IMAGE_MINIMAL=$(BASE_IMAGE_MINIMAL)

IMAGE_BUILD_ARGS_FULL = --target full \
                	-t $(IMAGE_TAG) \
	    		$(foreach tag,$(IMAGE_EXTRA_TAGS),-t $(tag)) \
	    		$(IMAGE_BUILD_EXTRA_OPTS) ./

IMAGE_BUILD_ARGS_MINIMAL = --target minimal \
                	   -t $(IMAGE_TAG)-minimal \
	            	   $(foreach tag,$(IMAGE_EXTRA_TAGS),-t $(tag)-minimal) \
	            	   $(IMAGE_BUILD_EXTRA_OPTS) ./

all: image

build:
	@mkdir -p bin
	$(GO_CMD) build -v -o bin $(LDFLAGS) ./cmd/...

install:
	$(GO_CMD) install -v $(LDFLAGS) ./cmd/...

image: yamls
	$(IMAGE_BUILD_CMD) $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_FULL)
	$(IMAGE_BUILD_CMD) $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_MINIMAL)

image-all: ensure-buildx yamls
# --load : not implemented yet, see: https://github.com/docker/buildx/issues/59
	$(IMAGE_BUILDX_CMD) $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_FULL)
	$(IMAGE_BUILDX_CMD) $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_MINIMAL)


# clean NFD labels on all nodes
# devel only
deploy-prune:
	kubectl apply -k deployment/overlays/prune/
	kubectl wait --for=condition=complete job -l app=nfd -n node-feature-discovery
	kubectl delete -k deployment/overlays/prune/


yamls:
	@./scripts/kustomize.sh $(K8S_NAMESPACE) $(IMAGE_REPO) $(IMAGE_TAG_NAME)

deploy: yamls
	kubectl apply -k .

templates:
	@# Need to prepend each line in the sample config with spaces in order to
	@# fit correctly in the configmap spec.
	@sed s'/^/    /' deployment/components/worker-config/nfd-worker.conf.example > nfd-worker.conf.tmp
	@# The sed magic below replaces the block of text between the lines with start and end markers
	@start=NFD-WORKER-CONF-START-DO-NOT-REMOVE; \
	end=NFD-WORKER-CONF-END-DO-NOT-REMOVE; \
	sed -e "/$$start/,/$$end/{ /$$start/{ p; r nfd-worker.conf.tmp" \
	    -e "}; /$$end/p; d }" -i deployment/helm/node-feature-discovery/values.yaml
	@rm nfd-worker.conf.tmp

generate:
	go mod vendor
	go generate ./cmd/... ./pkg/... ./source/...
	rm -rf vendor/
	controller-gen object crd output:crd:stdout paths=./pkg/apis/... > deployment/base/nfd-crds/nodefeaturerule-crd.yaml
	cp deployment/base/nfd-crds/nodefeaturerule-crd.yaml deployment/helm/node-feature-discovery/manifests/
	rm -rf sigs.k8s.io
	$(K8S_CODE_GENERATOR)/generate-groups.sh client,informer,lister \
	    sigs.k8s.io/node-feature-discovery/pkg/generated \
	    sigs.k8s.io/node-feature-discovery/pkg/apis \
	    "nfd:v1alpha1" --output-base=. \
	    --go-header-file hack/boilerplate.go.txt
	rm -rf pkg/generated
	mv sigs.k8s.io/node-feature-discovery/pkg/generated pkg/
	rm -rf sigs.k8s.io

gofmt:
	@$(GO_FMT) -w -l $$(find . -name '*.go')

gofmt-verify:
	@out=`$(GO_FMT) -w -l -d $$(find . -name '*.go')`; \
	if [ -n "$$out" ]; then \
	    echo "$$out"; \
	    exit 1; \
	fi

ci-lint:
	golangci-lint run --timeout 10m

lint:
	golint -set_exit_status ./...

mdlint:
	find docs/ -path docs/vendor -prune -false -o -name '*.md' | xargs $(MDL) -s docs/mdl-style.rb

helm-lint:
	helm lint --strict deployment/helm/node-feature-discovery/

test:
	$(GO_CMD) test ./cmd/... ./pkg/... ./source/...

e2e-test:
	@if [ -z ${KUBECONFIG} ]; then echo "[ERR] KUBECONFIG missing, must be defined"; exit 1; fi
	$(GO_CMD) test -v ./test/e2e/ -args -nfd.repo=$(IMAGE_REPO) -nfd.tag=$(IMAGE_TAG_NAME) \
	    -kubeconfig=$(KUBECONFIG) -nfd.e2e-config=$(E2E_TEST_CONFIG) -ginkgo.focus="\[kubernetes-sigs\]" \
	    $(if $(OPENSHIFT),-nfd.openshift,)
	$(GO_CMD) test -v ./test/e2e/ -args -nfd.repo=$(IMAGE_REPO) -nfd.tag=$(IMAGE_TAG_NAME)-minimal \
	    -kubeconfig=$(KUBECONFIG) -nfd.e2e-config=$(E2E_TEST_CONFIG) -ginkgo.focus="\[kubernetes-sigs\]" \
	    $(if $(OPENSHIFT),-nfd.openshift,)

push:
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)-minimal
	for tag in $(IMAGE_EXTRA_TAGS); do $(IMAGE_PUSH_CMD) $$tag; $(IMAGE_PUSH_CMD) $$tag-minimal; done

push-all: ensure-buildx yamls
	$(IMAGE_BUILDX_CMD) --push $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_FULL)
	$(IMAGE_BUILDX_CMD) --push $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_MINIMAL)

poll-images:
	set -e; \
	tags="$(foreach tag,$(IMAGE_TAG_NAME) $(IMAGE_EXTRA_TAG_NAMES),$(tag) $(tag)-minimal)" \
	base_url=`echo $(IMAGE_REPO) | sed -e s'!\([^/]*\)!\1/v2!'`; \
	for tag in $$tags; do \
	    image=$(IMAGE_REPO):$$tag \
	    errors=`curl -fsS -X GET https://$$base_url/manifests/$$tag|jq .errors`;  \
	    if [ "$$errors" = "null" ]; then \
	      echo Image $$image found; \
	    else \
	      echo Image $$image not found; \
	      exit 1; \
	    fi; \
	done

site-build:
	@mkdir -p docs/vendor/bundle
	$(SITE_BUILD_CMD) sh -c "bundle install && jekyll build $(JEKYLL_OPTS)"

site-serve:
	@mkdir -p docs/vendor/bundle
	$(SITE_BUILD_CMD) sh -c "bundle install && jekyll serve $(JEKYLL_OPTS) -H 127.0.0.1"
