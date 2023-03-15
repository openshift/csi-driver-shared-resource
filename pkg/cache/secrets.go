package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/config"
)

/*
Some old fashioned comments that describe what we are doing in this golang file.

First, some notes on cardinality:
- 1 share references 1 and only 1 secret currently
- given share related cardinality, many pods can reference a given secret via share CSI Volumes

Second, events.  We process Add and Update secret events from the controller in the same way, so we have an UpsertSecret function.
For delete events, the DelSecret is called.

On the use of sync.Map, see the comments in share.go

*/

var (
	// secretUpsertCallbacks has a key of the CSI volume ID and a value of the function to be called when a given
	// secret is updated, assuming the driver has mounted a share CSI volume with the configmap in a pod somewhere, and
	// the corresponding storage on the pod gets updated by the function that is the value of the entry.  Otherwise,
	// this map is empty and configmap updates result in a no-op.  This map is used both when we get an event for a given
	// secret or a series of events as a result of a relist from the controller.
	secretUpsertCallbacks = sync.Map{}
	// same thing as secretUpsertCallbacks but deletion of secrets, and of of course the controller relist does not
	// come into play here.
	secretDeleteCallbacks = sync.Map{}
)

// UpsertSecret adds or updates as needed the secret to our various maps for correlating with SharedSecrets and
// calls registered upsert callbacks
func UpsertSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	klog.V(6).Infof("UpsertSecret key %s", key)
	// first, find the shares pointing to this secret, and call the callbacks, in case certain pods
	// have had their permissions revoked; this will also handle if we had share events arrive before
	// the corresponding secret
	sharedSecretsList := client.ListSharedSecrets()
	for _, share := range sharedSecretsList {
		if share.Spec.SecretRef.Namespace == secret.Namespace && share.Spec.SecretRef.Name == secret.Name {
			shareSecretsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		}
	}

	// otherwise process any share that arrived after the secret
	secretUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
}

// DelSecret deletes this secret from the various secret related maps
func DelSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	klog.V(4).Infof("DelSecret key %s", key)
	secretDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
}

// RegisterSecretUpsertCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// if the corresponding share references a secret, then the function registered here will be called to possibly change
// storage
func RegisterSecretUpsertCallback(volID, sID string, f func(key, value interface{}) bool) {
	if !config.LoadedConfig.RefreshResources {
		return
	}
	secretUpsertCallbacks.Store(volID, f)
	ns, name, _ := SplitKey(sID)
	s := client.GetSecret(ns, name)
	if s != nil {
		f(BuildKey(s.Namespace, s.Name), s)
	} else {
		klog.Warningf("not found on secret with key %s vol %s", sID, volID)
	}
}

// UnregisterSecretUpsertCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSecretUpsertCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

// RegisterSecretDeleteCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// it records the CSI driver function to be called when a secret is deleted, so that the CSI
// driver can remove any storage mounted in the pod for the given secret
func RegisterSecretDeleteCallback(volID string, f func(key, value interface{}) bool) {
	secretDeleteCallbacks.Store(volID, f)
}

// UnregisterSecretDeleteCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSecretDeleteCallback(volID string) {
	secretDeleteCallbacks.Delete(volID)
}
