# Allows overriding of the container runtime
CONTAINER_RUNTIME ?= docker
# Allows overriding of the registry to push the generated image to
REGISTRY ?= quay.io
# Allows overriding of the repository to push the image to
REPOSITORY ?= openshift
# Allows overriding of the tag to generate for the image
TAG ?= latest

LDFLAGS ?= '-extldflags "-static"'

.DEFAULT_GOAL := help

all: clean generate verify build test
.PHONY: all

generate:
	./hack/update-generated.sh
.PHONY: generate

generate-crd:
	./hack/update-crd.sh

test: ## Run unit tests. Example: make test
	go test -race -count 1 ./cmd/... ./pkg/...
.PHONY: test

test-e2e:
	# for local testing set IMAGE_NAME to whatever image you produced via 'make build-image'
	# the test code will adjust the image supplied to the daemonset hostpath container
	./hack/test-e2e.sh
.PHONY: test-e2e

test-e2e-slow:
	# for local testing set IMAGE_NAME to whatever image you produced via 'make build-image'
	# the test code will adjust the image supplied to the daemonset hostpath container
	TEST_SUITE="slow" ./hack/test-e2e.sh
.PHONY: test-e2e

test-e2e-disruptive:
	# for local testing set IMAGE_NAME to whatever image you produced via 'make build-image'
	# the test code will adjust the image supplied to the daemonset hostpath container
	TEST_SUITE="disruptive" ./hack/test-e2e.sh
.PHONY: test-e2e

verify: ## Run verifications. Example: make verify
	go vet ./cmd/... ./pkg/... ./test/...
	gofmt -w ./cmd/ ./pkg/ ./test/
.PHONY: verify

build: ## Build the executable. Example: make build
	go build -a -mod=vendor -race -ldflags $(LDFLAGS) -o _output/csi-driver-shared-resource ./cmd
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

generate-release-yaml: ## Create single file with the relevant yaml from the deploy directory to facilitate deployment from the repository's release page
	./hack/generate-release-yaml.sh

.PHONY: generate-release-yaml

help: ## Print this help. Example: make help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
.PHONY: help