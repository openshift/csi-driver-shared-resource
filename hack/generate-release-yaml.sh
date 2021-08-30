#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

FILES=(
"00-namespace.yaml"
"01-service-account.yaml"
"02-cluster-role.yaml"
"03-cluster-role-binding.yaml"
"04-csi-hostpath-driverinfo.yaml"
"05-csi-hostpath-plugin.yaml"
)

rm -f release.yaml

for FILE in ${FILES[@]}
do

	echo -e "\n---\n" >> release.yaml
	cat deploy/${FILE} >> release.yaml

done
cat ./vendor/github.com/openshift/api/storage/v1alpha1/0000_10_sharedresource.crd.yaml >> release.yaml