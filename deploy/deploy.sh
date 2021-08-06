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

# BASE_DIR will default to deploy
BASE_DIR="deploy"
DEPLOY_DIR="_output/deploy"

function run () {
    echo "$@" >&2
    "$@"
}

echo "Creating deploy directory"
rm -rf "${DEPLOY_DIR}"
mkdir -p "${DEPLOY_DIR}"

defaultRegistrarImage="quay.io/openshift/origin-csi-node-driver-registrar:4.8.0"
registrarImage=${NODE_REGISTRAR_IMAGE:-${defaultRegistrarImage}}
defaultDriverImage="quay.io/openshift/origin-csi-driver-projected-resource:4.8.0"
driverImage=${DRIVER_IMAGE:-${defaultDriverImage}}

cp -r "${BASE_DIR}"/* "${DEPLOY_DIR}"

echo "Deploying using node driver registrar ${registrarImage}"
echo "Deploying using csi driver ${driverImage}"

# Replace the image refs in the canonical deployment YAML with env var overrides
sed -i -e "s|${defaultRegistrarImage}|${registrarImage}|g" \
  -e "s|${defaultDriverImage}|${driverImage}|g" \
  "${DEPLOY_DIR}/csi-hostpath-plugin.yaml"

# deploy hostpath plugin and registrar sidecar
echo "deploying csi driver components"
for i in $(find "${DEPLOY_DIR}"/*.yaml | sort); do
    echo "   $i"
    run oc apply -f "$i"
done

# Wait until all pods are running.
expected_running_pods=3
cnt=0
while [ "$(oc get pods -n csi-driver-projected-resource 2>/dev/null | grep -c '^csi-hostpath.* Running ')" -lt ${expected_running_pods} ]; do
    if [ $cnt -gt 30 ]; then
        echo "$(oc get pods 2>/dev/null | grep -c '^csi-hostpath.* Running ') running pods:"
        oc describe pods

        echo >&2 "ERROR: hostpath deployment not ready after over 5min"
        exit 1
    fi
    echo "$(date +%H:%M:%S) waiting for hostpath deployment to complete, attempt #$cnt"
    cnt=$((cnt + 1))
    sleep 10
done

