.PHONY: all test templates yamls build build-%
.FORCE:

GO_CMD ?= go
GO_FMT ?= gofmt

IMAGE_BUILD_CMD ?= docker build
IMAGE_BUILD_EXTRA_OPTS ?=
IMAGE_PUSH_CMD ?= docker push
CONTAINER_RUN_CMD ?= docker run
BUILDER_IMAGE ?= golang:1.24-bookworm
BASE_IMAGE_FULL ?= debian:bookworm-slim
BASE_IMAGE_MINIMAL ?= scratch

# Docker base command for working with html documentation.
# Use host networking because 'jekyll serve' is stupid enough to use the
# same site url than the "host" it binds to. Thus, all the links will be
# broken if we'd bind to 0.0.0.0
RUBY_IMAGE_VERSION := 3.3
JEKYLL_ENV ?= development
SITE_BUILD_CMD := $(CONTAINER_RUN_CMD) --rm -i -u "`id -u`:`id -g`" \
	$(shell [ -t 0 ] && echo '-t') \
	-e JEKYLL_ENV=$(JEKYLL_ENV) \
	-e JEKYLL_GITHUB_TOKEN="$$JEKYLL_GITHUB_TOKEN" \
	$(shell [ "$(JEKYLL_ENV)" = "development" ] && echo '-e PAGES_DISABLE_NETWORK=1') \
	--volume="$$PWD/docs:/work" \
	--volume="$$PWD/docs/vendor/bundle:/usr/local/bundle" \
	-w /work \
	--network=host ruby:$(RUBY_IMAGE_VERSION)
SITE_BASEURL ?=
SITE_DESTDIR ?= _site
JEKYLL_OPTS := -d '$(SITE_DESTDIR)' $(if $(SITE_BASEURL),-b '$(SITE_BASEURL)',)

VERSION := $(shell git describe --tags --dirty --always --match "v*")

CHART_VERSION ?= $(shell echo $(VERSION) | cut -c2-)

IMAGE_REGISTRY ?= registry.k8s.io/nfd
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

KUBECONFIG ?= ${HOME}/.kube/config
E2E_TEST_CONFIG ?=
E2E_PULL_IF_NOT_PRESENT ?= false
E2E_TEST_FULL_IMAGE ?= false
E2E_GINKGO_LABEL_FILTER ?=

BUILD_FLAGS = -tags osusergo,netgo \
              -ldflags "-s -w -extldflags=-static -X sigs.k8s.io/node-feature-discovery/pkg/version.version=$(VERSION) -X sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath.pathPrefix=$(HOSTMOUNT_PREFIX)"

# multi-arch build with buildx
IMAGE_ALL_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7

# enable buildx
ensure-buildx:
	./hack/init-buildx.sh

IMAGE_BUILDX_CMD ?= DOCKER_CLI_EXPERIMENTAL=enabled docker buildx build --builder=nfd-builder --platform=${IMAGE_ALL_PLATFORMS} --progress=auto --pull

IMAGE_BUILD_ARGS = --build-arg VERSION=$(VERSION) \
                --build-arg HOSTMOUNT_PREFIX=$(CONTAINER_HOSTMOUNT_PREFIX) \
                --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
                --build-arg BASE_IMAGE_FULL=$(BASE_IMAGE_FULL) \
                --build-arg BASE_IMAGE_MINIMAL=$(BASE_IMAGE_MINIMAL)

IMAGE_BUILD_ARGS_FULL = --target full \
                        -t $(IMAGE_TAG)-full \
                        $(foreach tag,$(IMAGE_EXTRA_TAGS),-t $(tag)-full) \
                        $(IMAGE_BUILD_EXTRA_OPTS) ./

IMAGE_BUILD_ARGS_MINIMAL = --target minimal \
                           -t $(IMAGE_TAG) \
                           -t $(IMAGE_TAG)-minimal \
                           $(foreach tag,$(IMAGE_EXTRA_TAGS),-t $(tag) -t $(tag)-minimal) \
                           $(IMAGE_BUILD_EXTRA_OPTS) ./

all: image

BUILD_BINARIES := nfd-master nfd-worker nfd-topology-updater nfd-gc kubectl-nfd nfd

build-%:
	$(GO_CMD) build -v -o bin/ $(BUILD_FLAGS) ./cmd/$*

build:	$(foreach bin, $(BUILD_BINARIES), build-$(bin))

install-%:
	$(GO_CMD) install -v $(BUILD_FLAGS) ./cmd/$*

install:	$(foreach bin, $(BUILD_BINARIES), install-$(bin))

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
	@./hack/kustomize.sh $(K8S_NAMESPACE) $(IMAGE_REPO) $(IMAGE_TAG_NAME)

deploy: yamls
	kubectl apply -k .

templates:
	@# Need to prepend each line in the sample config with spaces in order to
	@# fit correctly in the configmap spec.
	@sed s'/^/    /' deployment/components/worker-config/nfd-worker.conf.example > nfd-worker.conf.tmp
	@sed s'/^/    /' deployment/components/master-config/nfd-master.conf.example > nfd-master.conf.tmp
	@sed s'/^/    /' deployment/components/topology-updater-config/nfd-topology-updater.conf.example > nfd-topology-updater.conf.tmp
	@# The sed magic below replaces the block of text between the lines with start and end markers
	@start=NFD-MASTER-CONF-START-DO-NOT-REMOVE; \
	end=NFD-MASTER-CONF-END-DO-NOT-REMOVE; \
	sed -e "/$$start/,/$$end/{ /$$start/{ p; r nfd-master.conf.tmp" \
	    -e "}; /$$end/p; d }" -i deployment/helm/node-feature-discovery/values.yaml
	@start=NFD-WORKER-CONF-START-DO-NOT-REMOVE; \
	end=NFD-WORKER-CONF-END-DO-NOT-REMOVE; \
	sed -e "/$$start/,/$$end/{ /$$start/{ p; r nfd-worker.conf.tmp" \
	    -e "}; /$$end/p; d }" -i deployment/helm/node-feature-discovery/values.yaml
	@start=NFD-TOPOLOGY-UPDATER-CONF-START-DO-NOT-REMOVE; \
	end=NFD-TOPOLOGY-UPDATER-CONF-END-DO-NOT-REMOVE; \
	sed -e "/$$start/,/$$end/{ /$$start/{ p; r nfd-topology-updater.conf.tmp" \
		-e "}; /$$end/p; d }" -i deployment/helm/node-feature-discovery/values.yaml
	@rm nfd-master.conf.tmp
	@rm nfd-worker.conf.tmp
	@rm nfd-topology-updater.conf.tmp

.generator.image.stamp: Dockerfile_generator
	$(IMAGE_BUILD_CMD) \
	    --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
	    -t nfd-generator \
	    -f Dockerfile_generator .

generate: .generator.image.stamp
	$(CONTAINER_RUN_CMD) --rm \
	    -v "`pwd`:/go/node-feature-discovery" \
	    -v "`go env GOCACHE`:/.cache" \
	    -v "`go env GOMODCACHE`:/go/pkg/mod" \
	    --user=`id -u`:`id -g`\
	    nfd-generator \
	    ./hack/update_codegen.sh

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
	${CONTAINER_RUN_CMD} \
	--rm \
	--volume "${PWD}:/workdir:ro,z" \
	--workdir /workdir \
	ruby:slim \
	/workdir/scripts/test-infra/mdlint.sh

helm-lint:
	helm lint --strict deployment/helm/node-feature-discovery/

helm-push:
	helm package deployment/helm/node-feature-discovery --version $(CHART_VERSION) --app-version $(IMAGE_TAG_NAME)
	helm push node-feature-discovery-$(CHART_VERSION).tgz oci://${IMAGE_REGISTRY}/charts

test:
	$(GO_CMD) test -covermode=atomic -coverprofile=coverage.out ./cmd/... ./pkg/... ./source/...
	cd api/nfd && $(GO_CMD) test -covermode=atomic -coverprofile=coverage.out ./...

e2e-test:
	@if [ -z ${KUBECONFIG} ]; then echo "[ERR] KUBECONFIG missing, must be defined"; exit 1; fi
	$(GO_CMD) test -timeout=1h -v ./test/e2e/ -args \
	    -nfd.repo=$(IMAGE_REPO) -nfd.tag=$(IMAGE_TAG_NAME) \
	    -kubeconfig=$(KUBECONFIG) \
	    -nfd.e2e-config=$(E2E_TEST_CONFIG) \
	    -nfd.pull-if-not-present=$(E2E_PULL_IF_NOT_PRESENT) \
	    -ginkgo.focus="\[k8s-sigs\/node-feature-discovery\]" \
	    -ginkgo.label-filter=$(E2E_GINKGO_LABEL_FILTER) \
	    -ginkgo.v \
	    $(if $(OPENSHIFT),-nfd.openshift,)
	if [ "$(E2E_TEST_FULL_IMAGE)" = "true" ]; then \
	    $(GO_CMD) test -timeout=1h -v ./test/e2e/ -args \
	        -nfd.repo=$(IMAGE_REPO) -nfd.tag=$(IMAGE_TAG_NAME)-full \
	        -kubeconfig=$(KUBECONFIG) \
	        -nfd.e2e-config=$(E2E_TEST_CONFIG) \
	        -nfd.pull-if-not-present=$(E2E_PULL_IF_NOT_PRESENT) \
	        -ginkgo.focus="\[k8s-sigs\/node-feature-discovery\]" \
	        -ginkgo.label-filter=$(E2E_GINKGO_LABEL_FILTER) \
	        -ginkgo.v \
	        $(if $(OPENSHIFT),-nfd.openshift,); \
	fi

push:
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)-minimal
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)-full
	for tag in $(IMAGE_EXTRA_TAGS); do \
	    $(IMAGE_PUSH_CMD) $$tag; \
	    $(IMAGE_PUSH_CMD) $$tag-minimal; \
	    $(IMAGE_PUSH_CMD) $$tag-full; \
	done

push-all: ensure-buildx yamls
	$(IMAGE_BUILDX_CMD) --push $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_FULL)
	$(IMAGE_BUILDX_CMD) --push $(IMAGE_BUILD_ARGS) $(IMAGE_BUILD_ARGS_MINIMAL)

poll-images:
	set -e; \
	tags="$(foreach tag,$(IMAGE_TAG_NAME) $(IMAGE_EXTRA_TAG_NAMES),$(tag) $(tag)-minimal $(tag)-full)" \
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

benchmark:
	go test -bench=./pkg/nfd-master -run=^# ./pkg/nfd-master
