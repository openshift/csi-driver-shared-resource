package cache

import (
	"k8s.io/klog/v2"
	"sync"

	corev1 "k8s.io/api/core/v1"

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
	klog.V(4).Infof("AddShare key %s kind %s", key, br.Kind)
	switch br.Kind {
	case "ConfigMap":
		obj, ok := configmaps.Load(key)
		if obj != nil && ok {
			cm := obj.(*corev1.ConfigMap)
			configmapsWithShares.Store(key, cm)
			shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
			//NOTE we do not store share in shares unless the backing resource is available
			shares.Store(share.Name, share)
		} else {
			sharesWaitingOnConfigmaps.Store(share.Name, share)
		}
	case "Secret":
		obj, ok := secrets.Load(key)
		if obj != nil && ok {
			s := obj.(*corev1.Secret)
			secretsWithShare.Store(key, s)
			shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
			//NOTE we do not store share in shares unless the backing resource is available
			shares.Store(share.Name, share)
		} else {
			sharesWaitingOnSecrets.Store(share.Name, share)
		}
	}
}

func UpdateShare(share *sharev1alpha1.Share) {
	klog.V(4).Infof("UpdateShare key %s kind %s", share.Name, share.Spec.BackingResource.Kind)
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
	klog.V(4).Infof("UpdateShare key %s kind %s diff %v", share.Name, share.Spec.BackingResource.Kind, diffInstance)
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
	klog.V(4).Infof("DelShare key %s kind %s", key, br.Kind)
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
