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
	go test ./cmd/... ./pkg/...
.PHONY: test

verify: ## Run verifications. Example: make verify
	go vet ./cmd/... ./pkg/...
	gofmt -w ./cmd/ ./pkg/
.PHONY: verify

build: ## Build the executable. Example: make build
	go build -a -mod=vendor -race -ldflags $(LDFLAGS) -o _output/csi-driver-projected-resource ./cmd
.PHONY: build

build-image: ## Build the images and push them to the remote registry. Example: make build-images
	rm -rf _output
	$(CONTAINER_RUNTIME) build -f Dockerfile -t $(REGISTRY)/$(REPOSITORY)/origin-csi-driver-projected-resource:$(TAG) .
	$(CONTAINER_RUNTIME) push $(REGISTRY)/$(REPOSITORY)/origin-csi-driver-projected-resource:$(TAG)
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