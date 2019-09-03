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

GOFMT_CHECK=$(shell find . -not \( \( -wholename './.*' -o -wholename '*/vendor/*' \) -prune \) -name '*.go' | sort -u | xargs gofmt -s -l)

yaml_templates := $(wildcard *.yaml.template)
yaml_instances := $(patsubst %.yaml.template,%.yaml,$(yaml_templates))

all: image

image: yamls
	$(IMAGE_BUILD_CMD) --build-arg NFD_VERSION=$(VERSION) \
		-t $(IMAGE_TAG) \
		$(IMAGE_BUILD_EXTRA_OPTS) ./

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

verify: verify-gofmt

verify-gofmt:
ifeq (, $(GOFMT_CHECK))
	@echo "verify-gofmt: OK"
else
	@echo "verify-gofmt: ERROR: gofmt failed on the following files:"
	@echo "$(GOFMT_CHECK)"
	@echo ""
	@echo "For details, run: gofmt -d -s $(GOFMT_CHECK)"
	@echo ""
	@exit 1
endif

push:
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)
