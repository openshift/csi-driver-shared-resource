package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
)

var (
	configmaps               = sync.Map{}
	configmapUpsertCallbacks = sync.Map{}
	configmapDeleteCallbacks = sync.Map{}
	configmapsWithShares     = sync.Map{}
)

func UpsertConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	configmaps.Store(key, configmap)
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
