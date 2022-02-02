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
        - $2
      resourceNames:
        - $3
        - $4
      verbs:
        - use
EOF
}

if [ "${1}" == "sharedconfigmap" ]; then
  # create roles with shared configmap
  create_role sharedconfigmaps "" my-shared-config ""
elif [ "${1}" == "sharedsecret" ]; then
  # create roles with shared secret
  create_role sharedsecrets "" my-shared-secret ""
else
  # create roles with shared configmaps and secrets
  create_role sharedconfigmaps sharedsecrets my-shared-config my-shared-secret
fi

# create role binding
oc create rolebinding shared-resource-my-shared --role=shared-resource-my-shared --serviceaccount="$(oc project -q)":default
