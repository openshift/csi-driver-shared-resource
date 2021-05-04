#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

FILES=(
"00-namespace.yaml"
"0000_10_projectedresource.crd.yaml"
"01-service-account.yaml"
"02-cluster-role.yaml"
"03-cluster-role-binding.yaml"
"csi-hostpath-driverinfo.yaml"
"csi-hostpath-plugin.yaml"
)

rm -f release.yaml

for FILE in ${FILES[@]}
do

	echo -e "\n---\n" >> release.yaml
	cat deploy/${FILE} >> release.yaml

done
