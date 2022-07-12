package util

import (
	"strings"

	"github.com/openshift/csi-driver-shared-resource/pkg/config"
)

// IsAuthorizedSharedResource verify the shared resource already exists under OCP pre-populated shared resource
func IsAuthorizedSharedResource(kind, resource, resourceRefKeyNm, resourceRefKeyNs string) bool {
	var (
		value  string
		exists bool
	)
	if strings.HasPrefix(resource, "openshift-") {
		switch kind {
		case "SharedSecret":
			value, exists = config.LoadedSharedSecrets.AuthorizedSharedResources[resource]
		case "SharedConfigMap":
			value, exists = config.LoadedSharedConfigMaps.AuthorizedSharedResources[resource]
		}
		if !exists || value != (resourceRefKeyNs+":"+resourceRefKeyNm) {
			return false
		}
	}
	return true
}
