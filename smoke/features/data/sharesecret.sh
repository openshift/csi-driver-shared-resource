#!/bin/bash
set -x

create_shared_secret(){
  oc create -f - <<EOF
  kind: SharedSecret
  apiVersion: sharedresource.openshift.io/v1alpha1
  metadata:
    name: my-shared-secret
  spec:
    secretRef:
      namespace: $1
      name: my-secret
EOF
}

create_shared_secret "${1}"