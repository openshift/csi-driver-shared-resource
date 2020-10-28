package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
)

var (
	secrets               = sync.Map{}
	secretUpsertCallbacks = sync.Map{}
	secretDeleteCallbacks = sync.Map{}
	secretsWithShare      = sync.Map{}
)

func GetSecret(key interface{}) *corev1.Secret {
	obj, loaded := secretsWithShare.Load(key)
	if loaded {
		s, _ := obj.(*corev1.Secret)
		return s
	}
	return nil
}

func UpsertSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	secrets.Store(key, secret)
	secretsWithShare.Store(key, secret)
	secretUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
}

func DelSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	secrets.Delete(key)
	secretDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
	secretsWithShare.Delete(key)
}

func RegisterSecretUpsertCallback(volID string, f func(key, value interface{}) bool) {
	secretUpsertCallbacks.Store(volID, f)
	secretsWithShare.Range(f)
}

func UnregisterSecretUpsertCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

func RegisterSecretDeleteCallback(volID string, f func(key, value interface{}) bool) {
	secretDeleteCallbacks.Store(volID, f)
}

func UnregisterSecretDeleteCallback(volID string) {
	secretDeleteCallbacks.Delete(volID)
}
