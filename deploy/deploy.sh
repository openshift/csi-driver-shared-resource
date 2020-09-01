#!/usr/bin/env bash

# This script captures the steps required to successfully
# deploy the plugin driver.  This should be considered
# authoritative and all updates for this process should be
# done here and referenced elsewhere.

# The script assumes that kubectl is available on the OS path
# where it is executed.

set -e
set -o pipefail

BASE_DIR=$(dirname "$0")

run () {
    echo "$@" >&2
    "$@"
}


# deploy hostpath plugin and registrar sidecar
echo "deploying hostpath components"
for i in $(ls ${BASE_DIR}/*.yaml | sort); do
    echo "   $i"
    run kubectl apply -f $i
done

# Wait until all pods are running.
expected_running_pods=3
cnt=0
while [ $(kubectl get pods -n csi-driver-projected-resource 2>/dev/null | grep '^csi-hostpath.* Running ' | wc -l) -lt ${expected_running_pods} ]; do
    if [ $cnt -gt 30 ]; then
        echo "$(kubectl get pods 2>/dev/null | grep '^csi-hostpath.* Running ' | wc -l) running pods:"
        kubectl describe pods

        echo >&2 "ERROR: hostpath deployment not ready after over 5min"
        exit 1
    fi
    echo $(date +%H:%M:%S) "waiting for hostpath deployment to complete, attempt #$cnt"
    cnt=$(($cnt + 1))
    sleep 10
done

