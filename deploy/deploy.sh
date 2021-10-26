#!/usr/bin/env bash

# This script captures the steps required to successfully
# deploy the plugin driver.  This should be considered
# authoritative and all updates for this process should be
# done here and referenced elsewhere.

# The script assumes that oc is available on the OS path
# where it is executed.

# The following environment variables can be used to swap the images that are deployed:
#
# - NODE_REGISTRAR_IMAGE - this is the node driver registrar. Defaults to quay.io/openshift/origin-csi-node-driver-registrar:4.10.0
# - DRIVER_IMAGE - this is the CSI driver image. Defaults to quay.io/openshift/origin-csi-driver-projected-resource:4.10.0

set -e
set -o pipefail

# when not empty it will run CSI driver with "--refreshresources=false" flag.
NO_REFRESH_RESOURCES="${1}"

# BASE_DIR will default to deploy
BASE_DIR="deploy"
DEPLOY_DIR="_output/deploy"

# path to kutomize file, should be insite the temporary directory created for the rollout
KUSTOMIZATION_FILE="${DEPLOY_DIR}/kustomization.yaml"
# target namespace where resources are deployed
NAMESPACE="openshift-cluster-csi-drivers"

function run () {
    echo "$@" >&2
    "$@"
}

# initialize a kustomization.yaml file, listing all other yaml files as resources.
function kustomize_init () {
    cat <<EOS > ${KUSTOMIZATION_FILE}
---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
EOS

  echo "images:" >> ${KUSTOMIZATION_FILE}
}

# adds the settings to generate the configuration ConfigMap, taking in consideration extra flag for
# refresh-resources mode.
function kustomize_config () {
  # when no-refresh-resources is enabled, it translates the setting back to the existing
  # configuration file
  if [ -n "${NO_REFRESH_RESOURCES}" ] ; then
    echo "# Patching ConfigMap to have 'refreshResources: false' attribute"
    echo -e "\nrefreshResources: false" >> "${DEPLOY_DIR}/config.yaml"
  fi

  # adding extra generator options to kustomize file, so it can create a ConfigMap with embedded
  # YAML configuration
  cat <<EOS >> ${KUSTOMIZATION_FILE}
generatorOptions:
  disableNameSuffixHash: true
  labels:
    app: shared-resource-csi-driver-node

configMapGenerator:
  - name: csi-driver-shared-resource-config
    files:
      - config.yaml
EOS
}

# uses `oc wait` to wait for CSI driver pod to reach condition ready.
function wait_for_pod () {
  oc --namespace="${NAMESPACE}" wait pod \
    --for="condition=Ready=true" \
    --selector="app=shared-resource-csi-driver-node" \
    --timeout="5m"
}

echo "# Creating deploy directory at '${DEPLOY_DIR}'"

rm -rf "${DEPLOY_DIR}" || true
mkdir -p "${DEPLOY_DIR}"

cp -r -v "${BASE_DIR}"/* "${DEPLOY_DIR}"

echo "# Customizing resources..."

# initializing kustomize and adding the all resource files it should use
kustomize_init

# preparing configuraiton to become a ConfigMap
kustomize_config

# deploy hostpath plugin and registrar sidecar
echo "# Deploying the shared resource driver config.yaml config map on namespace '${NAMESPACE}'"
run oc apply --namespace="${NAMESPACE}" --kustomize ${DEPLOY_DIR}/

# waiting for all pods to reach condition ready
echo "# Waiting for pods to be ready..."
wait_for_pod || sleep 15 && wait_for_pod
