# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


GIT_COMMIT ?= $(shell git rev-parse HEAD)

LDFLAGS?='-extldflags "-static"'
GO_FILES=$(shell go list ./...)

# we'll change this once we are producing images out of CI/ART
REPO=gmontero

.PHONY: all build clean

all: clean build test

test:
	go test $(GO_FILES)
verify:
	go vet $(GO_FILES)
	gofmt -w cmd/ pkg/
build:
	go build -a -ldflags $(LDFLAGS) -o _output/hostpathplugin ./cmd/hostpathplugin
	go build -a -ldflags $(LDFLAGS) -o _output/csi-node-driver-registrar ./cmd/csi-node-driver-registrar
build-images:
	# save some time setting up the docker build context by deleting this first.
	rm -rf _output
	docker build -f images/hostpathplugin/Dockerfile -t docker.io/$(REPO)/origin-projected-resource-csi-driver:latest .
	docker push docker.io/$(REPO)/origin-projected-resource-csi-driver:latest
	docker build -f images/csi-node-driver-registrar/Dockerfile -t docker.io/$(REPO)/origin-csi-node-driver-registrar:latest .
	docker push docker.io/$(REPO)/origin-csi-node-driver-registrar:latest
clean:
	rm -rf _output

.PHONY: mod
mod:
	go mod tidy
	go mod vendor

