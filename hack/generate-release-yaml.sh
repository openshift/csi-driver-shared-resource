#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


FILES=(
"vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedsecret.crd.yaml"
"vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedconfigmap.crd.yaml"
"deploy/00-namespace.yaml"
"deploy/01-service-account.yaml"
"deploy/02-cluster-role.yaml"
"deploy/03-cluster-role-binding.yaml"
"deploy/csi-hostpath-driverinfo.yaml"
"deploy/csi-hostpath-plugin.yaml"
)

rm -f release.yaml

for FILE in ${FILES[@]}
do

	echo -e "\n---\n" >> release.yaml
	cat ${FILE} >> release.yaml

done
