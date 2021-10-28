#!/usr/bin/env bash
#
# Creates the ConfigMap to hold CSI driver configuration file. When a non-empty first argument is
# informed, it will make sure the refresh-resources attributes is set to false.
#

set -eu
set -o pipefail

# when not empty it will run CSI driver using refresh-resources mode disabled
NO_REFRESH_RESOURCES="${1:-}"
REFRESH_RESOURCES_KEY="refreshResources"

# temporary directory and temporary configuration location
CONFIG_DIR="_output/config"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"

# target namespace and configmap name
NAMESPACE="openshift-cluster-csi-drivers"
CONFIGMAP_NAME="csi-driver-shared-resource-config"

# making sure a working directory exists, and copying the default configuration
[ -d "${CONFIG_DIR}" ] || mkdir -p "${CONFIG_DIR}"
cp -f ./config/config.yaml ${CONFIG_FILE}

# given the configuration file does not contain the attribute already, appending the YAML
# configuration with desired entry
if [ -n "${NO_REFRESH_RESOURCES}" ] ; then
  echo "# Patching configuration with 'refreshResources: false'"
  grep -q -v "^${REFRESH_RESOURCES_KEY}\:" ${CONFIG_FILE} && \
	  echo -e "\n${REFRESH_RESOURCES_KEY}: false" >> ${CONFIG_FILE}
fi

# generating the configmap payload via the dry-run mode, a single key is added with configuration
# file payload embeding the payload as string
echo "# Generating the ConfigMap contents..."
CONFIGMAP_PAYLOAD=$(oc create configmap ${CONFIGMAP_NAME} \
	--from-file="${CONFIG_FILE}" \
	--dry-run=client \
	--output=yaml 
)

echo "# Creating the ConfigMap '${CONFIGMAP_NAME}'..."
oc --namespace="${NAMESPACE}" apply --filename <(echo "${CONFIGMAP_PAYLOAD}")