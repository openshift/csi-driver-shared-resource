#!/bin/bash
set -x

create_shared_configmap(){
  oc create -f - <<EOF
  kind: SharedConfigMap
  apiVersion: sharedresource.openshift.io/v1alpha1
  metadata:
    name: my-shared-config
  spec:
    configMapRef:
      namespace: $1
      name: share-config
EOF
}

create_shared_configmap ${1}