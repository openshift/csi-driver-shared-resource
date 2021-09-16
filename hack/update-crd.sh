#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

rm -rf deploy/0000_10_sharedresource.crd.yaml

echo "If you do not have controller-gen installed visit https://github.com/openshift/kubernetes-sigs-controller-tools/releases"

controller-gen schemapatch:manifests=./pkg/api/sharedresource/v1alpha1  \
paths=./pkg/api/sharedresource/v1alpha1 \
output:dir=./deploy

