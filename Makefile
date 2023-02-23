# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= $(shell git rev-parse --short HEAD)
BUNDLE_VERSION ?= $(VERSION)

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# ghcr.io/raptor-ml/raptor-bundle:$VERSION and ghcr.io/raptor-ml/raptor-catalog:$VERSION.
IMAGE_BASE ?= ghcr.io/raptor-ml/raptor

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_BASE)-bundle:$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(shell echo ${BUNDLE_VERSION} | sed s/^v//) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Image URL to use all building/pushing image targets
CORE_IMG_BASE = $(IMAGE_BASE)-core
RUNTIME_IMG_BASE = $(IMAGE_BASE)-runtime
HISTORIAN_IMG_BASE = $(IMAGE_BASE)-historian

CONTEXT ?= kind-raptor
KUBECTL = kubectl --context='${CONTEXT}'

$(info $(shell tput setaf 3)Context: $(shell tput sgr0)$(CONTEXT))
$(info $(shell tput setaf 3)Version: $(shell tput sgr0)$(VERSION))
$(info $(shell tput setaf 3)Base Image: $(shell tput sgr0)$(IMAGE_BASE))
$(info $(shell tput setaf 3)Core Image: $(shell tput sgr0)$(CORE_IMG_BASE))
$(info $(shell tput setaf 3)Historian Image: $(shell tput sgr0)$(HISTORIAN_IMG_BASE))
$(info $(shell tput setaf 3)Bundle Image: $(shell tput sgr0)$(BUNDLE_IMG))
$(info )

.DEFAULT_GOAL := help

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.2

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: pre-build controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=core-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: pre-build
pre-build: ## Run pre-build actions
	$(MAKE) -C ./internal/plugins/modelservers/sagemaker-ack get-configs

.PHONY: generate
generate: pre-build controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: buf-build
buf-build: buf ## Build protobufs with buf
	$(BUF) export buf.build/grpc-ecosystem/grpc-gateway -o api/proto
	$(BUF) export buf.build/envoyproxy/protoc-gen-validate  -o api/proto
	cd api/proto && $(BUF) mod update
	cd api/proto && $(BUF) build
	cd api/proto && $(BUF) breaking --against .
	cd api/proto && $(BUF) generate
	cd api/proto/gen/go && go mod tidy

.PHONY: fmt
fmt: pre-build ## Run go fmt against code.
	go fmt ./...

.PHONY: test
test: manifests generate fmt lint envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: test-e2e
test-e2e: docker-build ## Run integration tests.
	go test -v -timeout 1h -tags e2e github.com/raptor-ml/raptor/internal/e2e --args -v 5 --build-tag=$(VERSION)

.PHONY: check-license-headers
check-license:  ## Check the licenses and the license header.
	./hack/check-headers-for-license.sh
	./hack/licenses-check-allowed.sh

.PHONY: lint
lint: pre-build generate fmt golangci-lint buf check-license ## Run linters
	$(GOLANGCI_LINT) run
	$(BUF) lint api/proto

.PHONY: apidiff
apidiff: go-apidiff ## Run the go-apidiff to verify any API differences compared with origin/master
	$(GO_APIDIFF) master --compare-imports --print-compatible --repo-path=.

##@ Build

STREAMING_VERSION ?= latest
LDFLAGS ?= -s -w
LDFLAGS += -X github.com/raptor-ml/raptor/internal/version.Version=$(VERSION)
LDFLAGS += -X github.com/raptor-ml/raptor/internal/plugins/builders/streaming.runnerImg=ghcr.io/raptor-ml/streaming-runner:$(STREAMING_VERSION)

.PHONY: build
build: generate ## Build core binary.
	go build -ldflags="${LDFLAGS}" -a -o bin/core cmd/core/*.go
	go build -ldflags="${LDFLAGS}" -a -o bin/historian cmd/historian/*.go
	go build -ldflags="${LDFLAGS}" -a -o bin/runtime cmd/runtime/*.go

.PHONY: run
run: manifests generate fmt lint ## Run a controller from your host.
	go run ./cmd/raptor/*

.PHONY: docker-build
docker-build: generate docker-build-runtimes ## Build docker images.
	DOCKER_BUILDKIT=1 docker build --build-arg LDFLAGS="${LDFLAGS}" --build-arg VERSION="${VERSION}" -t ${CORE_IMG_BASE}:${VERSION} -t ${CORE_IMG_BASE}:latest --target core .
	DOCKER_BUILDKIT=1 docker build --build-arg LDFLAGS="${LDFLAGS}" --build-arg VERSION="${VERSION}" -t ${HISTORIAN_IMG_BASE}:${VERSION} -t ${HISTORIAN_IMG_BASE}:latest --target historian .

.PHONY: docker-build-runtimes
docker-build-runtimes: ## Build docker images for runtimes.
	DOCKER_DEFAULT_PLATFORM=linux/amd64 DOCKER_BUILDKIT=1 docker build --build-arg VERSION="${VERSION}" \
		--build-arg BASE_PYTHON_IMAGE="python:3.11-alpine"\
		-t ${RUNTIME_IMG_BASE}:${VERSION}-python3.11 -t ${RUNTIME_IMG_BASE}:latest-python3.11 \
		-t ${RUNTIME_IMG_BASE}:${VERSION} -t ${RUNTIME_IMG_BASE}:latest \
		--target runtime -f ./runtime/Dockerfile .

	DOCKER_DEFAULT_PLATFORM=linux/amd64 DOCKER_BUILDKIT=1 docker build --build-arg VERSION="${VERSION}" \
		--build-arg BASE_PYTHON_IMAGE="python:3.10-alpine"\
		-t ${RUNTIME_IMG_BASE}:${VERSION}-python3.10 -t ${RUNTIME_IMG_BASE}:latest-python3.10 \
		--target runtime -f ./runtime/Dockerfile .

	DOCKER_DEFAULT_PLATFORM=linux/amd64 DOCKER_BUILDKIT=1 docker build --build-arg VERSION="${VERSION}" \
		--build-arg BASE_PYTHON_IMAGE="python:3.9-alpine"\
		-t ${RUNTIME_IMG_BASE}:${VERSION}-python3.9 -t ${RUNTIME_IMG_BASE}:latest-python3.9 \
		--target runtime -f ./runtime/Dockerfile .

	DOCKER_DEFAULT_PLATFORM=linux/amd64 DOCKER_BUILDKIT=1 docker build --build-arg VERSION="${VERSION}" \
		--build-arg BASE_PYTHON_IMAGE="python:3.8-alpine"\
		-t ${RUNTIME_IMG_BASE}:${VERSION}-python3.8 -t ${RUNTIME_IMG_BASE}:latest-python3.8 \
		--target runtime -f ./runtime/Dockerfile .

	DOCKER_DEFAULT_PLATFORM=linux/amd64 DOCKER_BUILDKIT=1 docker build --build-arg VERSION="${VERSION}" \
		--build-arg BASE_PYTHON_IMAGE="python:3.7-alpine"\
		-t ${RUNTIME_IMG_BASE}:${VERSION}-python3.7 -t ${RUNTIME_IMG_BASE}:latest-python3.7 \
		--target runtime -f ./runtime/Dockerfile .

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: update_images_pre
update_images_pre: ## Update images in the manifests.
	cd config/core && $(KUSTOMIZE) edit set image raptor-core=${CORE_IMG_BASE}:${VERSION}
	cd config/core && $(KUSTOMIZE) edit set image raptor-runtime=${RUNTIME_IMG_BASE}:${VERSION}-python3.11
	cd config/historian && $(KUSTOMIZE) edit set image raptor-historian=${HISTORIAN_IMG_BASE}:${VERSION}

.PHONY: update_images_post
.PHONY: update_images_post
update_images_post: ## Update images in the manifests.
	cd config/core && $(KUSTOMIZE) edit set image raptor-core=raptor-core:latest
	cd config/core && $(KUSTOMIZE) edit set image raptor-runtime=raptor-runtime:latest-python3.11
	cd config/historian && $(KUSTOMIZE) edit set image raptor-historian=raptor-historian:latest

.PHONY: deploy
deploy: manifests kustomize update_images_pre ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -
	$(MAKE) update_images_post

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: installer
installer: manifests kustomize update_images_pre ## Create a kustomization file for the installer.
	cp hack/installer_tpl.sh installer.sh
	chmod +x installer.sh
	$(KUSTOMIZE) build config/installer | base64 >> installer.sh
	$(MAKE) update_images_post

.PHONY: kind-load
kind-load: ## Load docker images into kind.
	kind load docker-image --name raptor ${CORE_IMG_BASE}:${VERSION}
	kind load docker-image --name raptor ${HISTORIAN_IMG_BASE}:${VERSION}
	kind load docker-image --name raptor ${RUNTIME_IMG_BASE}:${VERSION}-python3.11
	kind load docker-image --name raptor ${RUNTIME_IMG_BASE}:${VERSION}-python3.10
	kind load docker-image --name raptor ${RUNTIME_IMG_BASE}:${VERSION}-python3.9
	kind load docker-image --name raptor ${RUNTIME_IMG_BASE}:${VERSION}-python3.8
	kind load docker-image --name raptor ${RUNTIME_IMG_BASE}:${VERSION}-python3.7

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN): ## Ensure that the directory exists
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.9.2

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE):
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v4@latest

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN):
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST):
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

OSDK ?= $(LOCALBIN)/operator-sdk
OPERATOR_SDK_VERSION=v1.25.1

.PHONY: operator-sdk
operator-sdk: $(OSDK) ## Download controller-gen locally if necessary.
$(OSDK):
	curl -sSLo $(OSDK) https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}/operator-sdk_$(shell go env GOOS)_$(shell go env GOARCH) && chmod +x ${OSDK}

.PHONY: bundle
bundle: operator-sdk manifests kustomize update_images_pre ## Generate bundle manifests and metadata, then validate generated files.
	cd config/manifests/bases && $(KUSTOMIZE) edit set annotation containerImage:${CORE_IMG_BASE}:${VERSION}
	$(OSDK) generate kustomize manifests --apis-dir api/v1alpha1 -q
	$(KUSTOMIZE) build config/manifests | $(OSDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OSDK) bundle validate ./bundle --select-optional suite=operatorframework
	cd config/manifests/bases && $(KUSTOMIZE) edit set annotation containerImage:${CORE_IMG_BASE}:latest
	$(MAKE) update_images_post

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.19.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_BASE)-catalog:$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

##@ Tools

BUF ?= $(LOCALBIN)/buf
.PHONY: buf
buf: $(BUF)
$(BUF):
	GOBIN=$(LOCALBIN) go install github.com/bufbuild/buf/cmd/buf@latest

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION  = v1.50.1
.PHONY: golangci-lint
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
		set -e ;\
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

GO_APIDIFF = $(LOCALBIN)/go-apidiff
.PHONY: go-apidiff
go-apidiff:
	GOBIN=$(LOCALBIN) go install github.com/joelanford/go-apidiff@lates
