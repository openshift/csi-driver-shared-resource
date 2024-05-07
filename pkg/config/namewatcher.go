package config

import (
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const (
	sharedSecretReservedNamesEnvVarName    = "RESERVED_SHARED_SECRET_NAMES"
	sharedConfigMapReservedNamesEnvVarName = "RESERVED_SHARED_CONFIGMAP_NAMES"
)

type ReservedNames struct {
	currentSecretNames    map[string]types.NamespacedName
	currentConfigMapNames map[string]types.NamespacedName
}

func SetupNameReservation() *ReservedNames {
	rn := &ReservedNames{}
	sharedSecretReservations := os.Getenv(sharedSecretReservedNamesEnvVarName)
	rn.currentSecretNames = parseEnvVar(sharedSecretReservations, sharedSecretReservedNamesEnvVarName)
	klog.Infof("Loaded reserved shared secret from environment variable %s and data %#v", sharedSecretReservations, rn.currentSecretNames)
	sharedConfigMapReservations := os.Getenv(sharedConfigMapReservedNamesEnvVarName)
	rn.currentConfigMapNames = parseEnvVar(sharedConfigMapReservations, sharedConfigMapReservedNamesEnvVarName)
	klog.Infof("Loaded reserved shared configmap information from environment variable %s and data %#v", sharedConfigMapReservations, rn.currentConfigMapNames)
	return rn
}

func parseEnvVar(val, envvarName string) map[string]types.NamespacedName {
	ret := map[string]types.NamespacedName{}
	entries := strings.Split(val, ";")
	for _, entry := range entries {
		a := strings.Split(entry, ":")
		if len(a) != 3 {
			if len(entry) > 0 {
				klog.Warningf("env var %s has bad entry %s", envvarName, entry)
			}
			continue
		}
		nsName := types.NamespacedName{
			Namespace: strings.TrimSpace(a[1]),
			Name:      strings.TrimSpace(a[2]),
		}
		ret[strings.TrimSpace(a[0])] = nsName
	}
	return ret
}

func (rn *ReservedNames) ValidateSharedSecretOpenShiftName(shareName, refNamespace, refName string) bool {
	v, ok := rn.currentSecretNames[shareName]
	return innerValidate(shareName, refNamespace, refName, ok, v)
}

func (rn *ReservedNames) ValidateSharedConfigMapOpenShiftName(shareName, refNamespace, refName string) bool {
	v, ok := rn.currentConfigMapNames[shareName]
	return innerValidate(shareName, refNamespace, refName, ok, v)
}

func startsWithOpenShift(shareName string) bool {
	if strings.HasPrefix(shareName, "openshift") {
		return true
	}
	return false
}

func innerValidate(shareName, refNamespace, refName string, ok bool, v types.NamespacedName) bool {
	// TODO: Make this configurable, do not skip reservation logic if resource share name does not start with "openshift*".
	if !startsWithOpenShift(shareName) {
		return true
	}
	if !ok {
		return false
	}
	if strings.TrimSpace(v.Namespace) != strings.TrimSpace(refNamespace) ||
		strings.TrimSpace(v.Name) != strings.TrimSpace(refName) {
		return false
	}
	return true

}
