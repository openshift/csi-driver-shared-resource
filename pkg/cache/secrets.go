package cache

import (
	"sync"

	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var (
	secrets                = sync.Map{}
	secretUpsertCallbacks  = sync.Map{}
	secretDeleteCallbacks  = sync.Map{}
	secretsWithShare       = sync.Map{}
	sharesWaitingOnSecrets = sync.Map{}
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
	// in case share arrived before secret
	processedSharesWithoutSecrets := []string{}
	sharesWaitingOnSecrets.Range(func(key, value interface{}) bool {
		shareKey := key.(string)
		share := value.(*sharev1alpha1.Share)
		br := share.Spec.BackingResource
		secretKey := BuildKey(br.Namespace, br.Name)
		secretsWithShare.Store(secretKey, secret)
		//NOTE: share update ranger will store share in shares sync.Map
		shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		processedSharesWithoutSecrets = append(processedSharesWithoutSecrets, shareKey)
		return true
	})
	for _, shareKey := range processedSharesWithoutSecrets {
		sharesWaitingOnSecrets.Delete(shareKey)
	}
	// otherwise process any share that arrived after the secret
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
