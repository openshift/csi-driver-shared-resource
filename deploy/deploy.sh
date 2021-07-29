#!/usr/bin/env bash

# This script captures the steps required to successfully
# deploy the plugin driver.  This should be considered
# authoritative and all updates for this process should be
# done here and referenced elsewhere.

# The script assumes that oc is available on the OS path
# where it is executed.

set -e
set -o pipefail

BASE_DIR=$(dirname "$0")

run () {
    echo "$@" >&2
    "$@"
}


# deploy csi driver daemonset
echo "deploying daemonset components"
for i in $(ls ${BASE_DIR}/*.yaml | sort); do
    echo "   $i"
    run oc apply -f $i
done

# Wait until all pods are running.
expected_running_pods=3
cnt=0
while [ $(oc get pods -n csi-driver-shared-resource 2>/dev/null | grep '^csi-driver-shared-resource.* Running ' | wc -l) -lt ${expected_running_pods} ]; do
    if [ $cnt -gt 30 ]; then
        echo "$(oc get pods 2>/dev/null | grep '^csi-driver-shared-resource.* Running ' | wc -l) running pods:"
        oc describe pods

        echo >&2 "ERROR: daemonset deployment not ready after over 5min"
        exit 1
    fi
    echo $(date +%H:%M:%S) "waiting for daemonset deployment to complete, attempt #$cnt"
    cnt=$(($cnt + 1))
    sleep 10
done

