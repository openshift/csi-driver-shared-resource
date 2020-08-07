package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
)

var (
	secrets               = sync.Map{}
	secretUpsertCallbacks = sync.Map{}
	secretDeleteCallbacks = sync.Map{}
)

func UpsertSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	secrets.Store(key, secret)
	secretUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
}

func DelSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	secrets.Delete(key)
	secretDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
}

func RegisterSecretUpsertCallback(volID string, f func(key, value interface{}) bool) {
	secretUpsertCallbacks.Store(volID, f)
	secrets.Range(f)
}

func UnregisterSecretUpsertCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

func RegisterSecretDeleteCallback(volID string, f func(key, value interface{}) bool) {
	secretUpsertCallbacks.Store(volID, f)
}

func UnregisterSecretDeleteCallback(voldID string) {
	secretDeleteCallbacks.Delete(voldID)
}
