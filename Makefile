# Allows overriding of the container runtime
CONTAINER_RUNTIME ?= docker
# Allows overriding of the registry to push the generated image to
REGISTRY ?= quay.io
# Allows overriding of the repository to push the image to
REPOSITORY ?= openshift
# Allows overriding of the tag to generate for the image
TAG ?= latest

# when non-empty it enables "--refreshresource=false" deployment
DEPLOY_MODE ?=

# amount of pods the daemonset will produce before starting end-to-end tests
DAEMONSET_PODS ?= 3

# end-to-end test suite name
TEST_SUITE ?= normal
# ent-to-end test timeout
TEST_TIMEOUT ?= 30m

TARGET_GOOS ?= $(shell go env GOOS)
TARGET_GOARCH ?= $(shell go env GOARCH)

# For golang 1.16, race detection is only widely supported for amd64 architectures (linux, windows, darwin, and freebsd).
# Race detection for ARM is only currently supported for linux (no darwin or windows support yet).
# s390x and ppc64le do not support race detection at present.
ifeq ($(TARGET_GOARCH), amd64)
	RACE = -race
endif

GOFLAGS ?= -a -mod=vendor $(RACE)

.DEFAULT_GOAL := help

all: clean verify build test
.PHONY: all

test: ## Run unit tests. Example: make test
	env GOOS=$(TARGET_GOOS) GOARCH=$(TARGET_GOARCH) go test $(GOFLAGS) -count 1 ./cmd/... ./pkg/...
.PHONY: test

config: ## Overwrites the configuration ConfigMap with local settings
	./hack/configmap.sh $(DEPLOY_MODE)
.PHONY: config

test-e2e-no-deploy:
	TEST_SUITE=$(TEST_SUITE) TEST_TIMEOUT=$(TEST_TIMEOUT) DAEMONSET_PODS=$(DAEMONSET_PODS) ./hack/test-e2e.sh
.PHONY: test-e2e-no-deploy

test-e2e: test-e2e-no-deploy

test-e2e-no-refreshresources: TEST_SUITE = "norefresh"
test-e2e-no-refreshresources: test-e2e

test-e2e-slow: TEST_SUITE = "slow"
test-e2e-slow: test-e2e

test-e2e-disruptive: TEST_SUITE = "disruptive" 
test-e2e-disruptive: test-e2e

verify: ## Run verifications. Example: make verify
	go vet ./cmd/... ./pkg/... ./test/...
	gofmt -w ./cmd/ ./pkg/ ./test/
.PHONY: verify

build: ## Build the executable. Example: make build
	env GOOS=$(TARGET_GOOS) GOARCH=$(TARGET_GOARCH) go build $(GOFLAGS) -o _output/csi-driver-shared-resource ./cmd
.PHONY: build

build-image: ## Build the images and push them to the remote registry. Example: make build-image
	rm -rf _output
	$(CONTAINER_RUNTIME) build -f Dockerfile -t $(REGISTRY)/$(REPOSITORY)/origin-csi-driver-shared-resource:$(TAG) .
	$(CONTAINER_RUNTIME) push $(REGISTRY)/$(REPOSITORY)/origin-csi-driver-shared-resource:$(TAG)
.PHONY: build-image

clean: ## Clean up the workspace. Example: make clean
	rm -rf _output
.PHONY: clean

vendor: ## Vendor Go dependencies. Example: make vendor
	go mod tidy
	go mod vendor
.PHONY: vendor

help: ## Print this help. Example: make help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
.PHONY: help
