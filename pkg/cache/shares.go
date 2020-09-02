package cache

import (
	corev1 "k8s.io/api/core/v1"
	"sync"

	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
)

var (
	shares               = sync.Map{}
	shareUpdateCallbacks = sync.Map{}
	shareDeleteCallbacks = sync.Map{}
)

func AddShare(share *sharev1alpha1.Share) {
	br := share.Spec.BackingResource
	key := BuildKey(br.Namespace, br.Name)
	switch br.Kind {
	case "ConfigMap":
		obj, ok := configmaps.Load(key)
		if obj != nil && ok {
			cm := obj.(*corev1.ConfigMap)
			configmapsWithShares.Store(key, cm)
			shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		} else {
			configmapsWithShares.Store(key, key)
		}
	case "Secret":
		obj, ok := secrets.Load(key)
		if obj != nil && ok {
			s := obj.(*corev1.Secret)
			secretsWithShare.Store(key, s)
			shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		} else {
			secretsWithShare.Store(key, key)
		}
	}
}

func UpdateShare(share *sharev1alpha1.Share) {
	old, ok := shares.Load(share.Name)
	if !ok || old == nil {
		AddShare(share)
		return
	}
	oldShare := old.(*sharev1alpha1.Share)
	diffInstance := false
	oldBr := oldShare.Spec.BackingResource
	newBr := share.Spec.BackingResource
	switch {
	case oldBr.Kind != newBr.Kind:
		diffInstance = true
	case oldBr.Namespace != newBr.Namespace:
		diffInstance = true
	case oldBr.Name != newBr.Name:
		diffInstance = true
	}
	if !diffInstance {
		shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		return
	}

	shares.Store(share.Name, share)
	br := share.Spec.BackingResource
	key := BuildKey(br.Namespace, br.Name)
	configmapsWithShares.Delete(key)
	secretsWithShare.Delete(key)
	AddShare(share)
}

func DelShare(share *sharev1alpha1.Share) {
	br := share.Spec.BackingResource
	key := BuildKey(br.Namespace, br.Name)
	configmapsWithShares.Delete(key)
	secretsWithShare.Delete(key)
	shares.Delete(share.Name)
	shareDeleteCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
}

func RegisterShareUpdateCallback(volID string, f func(key, value interface{}) bool) {
	shareUpdateCallbacks.Store(volID, f)
	shares.Range(f)
}

func UnregisterShareUpdateCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

func RegisterShareDeleteCallback(volID string, f func(key, value interface{}) bool) {
	shareDeleteCallbacks.Store(volID, f)
}

func UnregisterShareDeleteCallback(volID string) {
	shareDeleteCallbacks.Delete(volID)
}
