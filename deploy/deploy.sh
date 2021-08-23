#!/usr/bin/env bash

# This script captures the steps required to successfully
# deploy the plugin driver.  This should be considered
# authoritative and all updates for this process should be
# done here and referenced elsewhere.

# The script assumes that oc is available on the OS path
# where it is executed.

# The following environment variables can be used to swap the images that are deployed:
#
# - NODE_REGISTRAR_IMAGE - this is the node driver registrar. Defaults to quay.io/openshift/origin-csi-node-driver-registrar:4.8.0
# - DRIVER_IMAGE - this is the CSI driver image. Defaults to quay.io/openshift/origin-csi-driver-projected-resource:4.8.0

set -e
set -o pipefail

# when not empty it will run CSI driver with "--refreshresources=false" flag.
NO_REFRESH_RESOURCES="${1}"

# customize images used by registrar and csi-driver containers
NODE_REGISTRAR_IMAGE="${NODE_REGISTRAR_IMAGE:-}"
DRIVER_IMAGE="${DRIVER_IMAGE:-}"

# BASE_DIR will default to deploy
BASE_DIR="deploy"
DEPLOY_DIR="_output/deploy"

KUSTOMIZATION_FILE="${DEPLOY_DIR}/kustomization.yaml"

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

  for f in $(find "${DEPLOY_DIR}"/*.yaml |grep -v kustomization |sort); do
    f=$(basename ${f})
    echo "## ${f}"
    echo "  - ${f}" >> ${KUSTOMIZATION_FILE}
  done

  echo "images:" >> ${KUSTOMIZATION_FILE}
}

# creates a new entry with informed name and target image. The target image is split on URL and tag.
function kustomize_set_image () {
  local NAME=${1}
  local TARGET=${2}

  # splitting target image in URL and tag
  IFS=':' read -a PARTS <<< ${TARGET}

  cat <<EOS >> ${KUSTOMIZATION_FILE}
  - name: ${NAME}
    newName: ${PARTS[0]}
    newTag: ${PARTS[1]}
EOS
}

# patches the CSI DaemonSet primary container to include one more argument.
function kustomize_add_arg () {
  local ARG=${1}
  local FILE="args-patch-${ARG}.json"
  cat <<EOS > ${DEPLOY_DIR}/${FILE}
[
  {
    "op": "add",
    "path": "/spec/template/spec/containers/1/args/-",
    "value": "${ARG}"
  }
]
EOS
  cat <<EOS >> ${KUSTOMIZATION_FILE}
patchesJson6902:
  - path: ${FILE}
    target:
      group: apps
      version: v1
      kind: DaemonSet
      namespace: openshift-cluster-csi-drivers
      name: csi-hostpathplugin
EOS
}

# uses `oc wait` to wait for CSI driver pod to reach condition ready.
function wait_for_pod () {
  oc --namespace="openshift-cluster-csi-drivers" wait pod \
    --for="condition=Ready=true" \
    --selector="app=csi-hostpathplugin" \
    --timeout="5m"
}

echo "# Creating deploy directory at '${DEPLOY_DIR}'"

rm -rf "${DEPLOY_DIR}" || true
mkdir -p "${DEPLOY_DIR}"

cp -r -v "${BASE_DIR}"/* "${DEPLOY_DIR}"

echo "# Customizing resources..."

# initializing kustomize and adding the all resource files it should use
kustomize_init

if [ ! -z "${NODE_REGISTRAR_IMAGE}" ] ; then
  echo "# Patching node-registrar image to use '${NODE_REGISTRAR_IMAGE}'"
  kustomize_set_image "quay.io/openshift/origin-csi-node-driver-registrar" "${NODE_REGISTRAR_IMAGE}"
fi

if [ ! -z "${DRIVER_IMAGE}" ] ; then
  echo "# Patching node-csi-driver image to use '${DRIVER_IMAGE}'"
  kustomize_set_image "quay.io/openshift/origin-csi-driver-shared-resource" "${DRIVER_IMAGE}"
fi

# adding to disable refresh-resources using kustomize-v3 approach (embedded on `oc`)
if [ ! -z "${NO_REFRESH_RESOURCES}" ] ; then
  echo "# Patching DaemonSet container to use '--refreshresources=false' flag"
  kustomize_add_arg "--refreshresources=false"
fi

# deploy hostpath plugin and registrar sidecar
echo "# Deploying csi driver components"
run oc apply --kustomize ${DEPLOY_DIR}/

# waiting for all pods to reach condition ready
echo "# Waiting for pods to be ready..."
wait_for_pod || sleep 15 && wait_for_pod
