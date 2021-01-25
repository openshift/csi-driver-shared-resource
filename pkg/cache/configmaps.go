package cache

import (
	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

var (
	configmaps                = sync.Map{}
	configmapUpsertCallbacks  = sync.Map{}
	configmapDeleteCallbacks  = sync.Map{}
	configmapsWithShares      = sync.Map{}
	sharesWaitingOnConfigmaps = sync.Map{}
)

func GetConfigMap(key interface{}) *corev1.ConfigMap {
	obj, loaded := configmapsWithShares.Load(key)
	if loaded {
		cm, _ := obj.(*corev1.ConfigMap)
		return cm
	}
	return nil
}

func UpsertConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	configmaps.Store(key, configmap)
	// in case share arrived before configmap
	processSharesWithoutConfigmaps := []string{}
	sharesWaitingOnConfigmaps.Range(func(key, value interface{}) bool {
		shareKey := key.(string)
		share := value.(*sharev1alpha1.Share)
		br := share.Spec.BackingResource
		configmapKey := BuildKey(br.Namespace, br.Name)
		configmapsWithShares.Store(configmapKey, configmap)
		//NOTE: share update ranger will store share in shares sync.Map
		shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		processSharesWithoutConfigmaps = append(processSharesWithoutConfigmaps, shareKey)
		return true
	})
	for _, shareKey := range processSharesWithoutConfigmaps {
		sharesWaitingOnConfigmaps.Delete(shareKey)
	}
	// otherwise process any share that arrived after the secret
	configmapsWithShares.Store(key, configmap)
	configmapUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, configmap)))
}

func DelConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	configmaps.Delete(key)
	configmapDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, configmap)))
	configmapsWithShares.Delete(key)
}

func RegisterConfigMapUpsertCallback(volID string, f func(key, value interface{}) bool) {
	configmapUpsertCallbacks.Store(volID, f)
	configmapsWithShares.Range(f)
}

func UnregisterConfigMapUpsertCallback(volID string) {
	configmapUpsertCallbacks.Delete(volID)
}

func RegisterConfigMapDeleteCallback(volID string, f func(key, value interface{}) bool) {
	configmapDeleteCallbacks.Store(volID, f)
}

func UnregisterConfigMapDeleteCallback(volID string) {
	configmapDeleteCallbacks.Delete(volID)
}
