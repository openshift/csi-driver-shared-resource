#!/bin/bash
set -x

create_role(){
  oc create -f - << EOF
  apiVersion: rbac.authorization.k8s.io/v1
  kind: Role
  metadata:
    name: shared-resource-my-shared
  rules:
    - apiGroups:
        - sharedresource.openshift.io
      resources:
        - $1
      resourceNames:
        - $2
      verbs:
        - use
EOF
}

if [ ${1} == "configmap" ]; then
  # create roles with shared configmap
  create_role sharedconfigmaps my-shared-config
else
  # create roles with shared secret
  create_role sharedsecrets my-shared-secret
fi

NAMESPACE = $(oc project -q)

# create role binding
oc create rolebinding shared-resource-my-shared --role=shared-resource-my-shared --serviceaccount=$NAMESPACE:default
