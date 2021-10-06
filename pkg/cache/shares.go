package cache

import (
	"sync"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
)

/*
Some old fashioned comments that describe what we are doing in this golang file.

First, some notes on cardinality:
- 1 share at the moment only references 1 configmap or 1 secret
- 1 share can be used by many pods, and in particular, many CSI Driver volume mounts across those pods.  This driver
end of the day manages those CSI volumes, with of course the associated Pod and its various fields key metadata

Second, events.  As the AddSharedConfigMap, UpdateSharedConfigMap, or DelSharedConfigMap names imply, those methods are called when the controller
processes Add/Update/Delete events for SharedResource instances.  To date, the Update path is a superset of the Add path.  Or
in other words, the UpdateSharedConfigMap ultimately calls the AddSharedConfigMap function.

Third, our data structure of note:  the golang sync.Map.  The implementation provides some key features for us:
- the implicit synchronization as we get, store, delete specific entries, or range over a set of entries
- with range in particular, you supply a function that is called as each key/value pair is inspected.  That function
receives the key and value, and as golang function parameters work, that function can be seeded with data specific to
where it was created, vs. its use here.  This allows us to abstract the functional details specific our CSI volume
implementation, and the events it receives from the kubelet as part of Pod creation, from the code here that deals with
handling share events from the controller
- so the CSI driver side of our solution here "registers callbacks".  Those "callbacks" are code on its side that it wants
executed when a share creation, update. or deletion event occurs.
- much like you'll see with data grid products, the "maps" are effectively in memory database tables, albeit with simple
indexing and querying (which is implemented by how we create map subsets "seed" the Range call).

Fourth, a couple of notes on permissions and shareConfigMaps
- The SAR execution occurs on 2 events:
- 1) the pod creation / request to mount the CSI volume event from the kubelet
- 2) when a share update event occurs, which can be when the share is actually updated, or on the relist; the relist
variety is how we currently verify that a given pod is *STILL* allowed to access a given share if nothing else has changed.

*/

var (
	// shareConfigMaps/shareSecrets is our global share ID (remember it is cluster scoped) to share map; generally, it facilitates lookup
	// when we are dealing with Pod creation events from the kubelet
	shareConfigMaps = sync.Map{}
	shareSecrets    = sync.Map{}
	// sharesUpdateCallbacks/shareSecretsUpdateCallbacks have a key of the CSI volume ID and a value of the function to be called when a given
	// share is to updated, assuming the driver has mounted a share CSI volume in a pod somewhere.  Otherwise,
	// this map is empty and share updates result in a no-op.  This map is used both when we get an event for a given
	// share or a series of events as a result of a relist from the controller.
	shareConfigMapsUpdateCallbacks = sync.Map{}
	shareSecretsUpdateCallbacks    = sync.Map{}
	// same thing as shareConfigMapsUpdateCallbacks/shareSecretsUpdateCallbacks, but deletion of the objects, and of course the controller relist does not
	// come into play here.
	shareConfigMapsDeleteCallbacks = sync.Map{}
	shareSecretsDeleteCallbacks    = sync.Map{}
)

// AddSharedConfigMap adds the SharedConfigMap and its referenced config map to our various tracking maps
func AddSharedConfigMap(share *sharev1alpha1.SharedConfigMap) {
	br := share.Spec.ConfigMapRef
	key := BuildKey(br.Namespace, br.Name)
	klog.V(4).Infof("AddSharedConfigMap key %s", key)
	obj, ok := configmaps.Load(key)
	if obj != nil && ok {
		cm := obj.(*corev1.ConfigMap)
		configmapsWithShares.Store(key, cm)
		// so this line build a map with a single entry, the share from this event, and then
		// applies the function(s) supplied by the CSI volume code in order to make changes based
		// on this event
		shareConfigMapsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		//NOTE we do not store share in shareConfigMaps unless the backing resource is available
		shareConfigMaps.Store(share.Name, share)
	} else {
		sharesWaitingOnConfigmaps.Store(share.Name, share)
	}
}

// AddSharedSecret adds the SharedSecret and its referenced secret to our various tracking maps
func AddSharedSecret(share *sharev1alpha1.SharedSecret) {
	br := share.Spec.SecretRef
	key := BuildKey(br.Namespace, br.Name)
	klog.V(4).Infof("AddSharedSecret key %s", key)
	obj, ok := secrets.Load(key)
	if obj != nil && ok {
		s := obj.(*corev1.Secret)
		secretsWithShare.Store(key, s)
		// so this line build a map with a single entry, the share from this event, and then
		// applies the function(s) supplied by the CSI volume code in order to make changes based
		// on this event
		shareSecretsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		//NOTE we do not store share in shareConfigMaps unless the backing resource is available
		shareSecrets.Store(share.Name, share)
	} else {
		sharesWaitingOnSecrets.Store(share.Name, share)
	}
}

// UpdateSharedConfigMap updates the SharedConfigMap in our various tracking maps and if need be calls
// the registered callbacks to update the content on any volumes using the SharedConfigMap
func UpdateSharedConfigMap(share *sharev1alpha1.SharedConfigMap) {
	klog.V(4).Infof("UpdateSharedConfigMap key %s", share.Name)
	old, ok := shareConfigMaps.Load(share.Name)
	if !ok || old == nil {
		AddSharedConfigMap(share)
		return
	}
	oldShare := old.(*sharev1alpha1.SharedConfigMap)
	diffInstance := false
	oldBr := oldShare.Spec.ConfigMapRef
	newBr := share.Spec.ConfigMapRef
	switch {
	case oldBr.Namespace != newBr.Namespace:
		diffInstance = true
	case oldBr.Name != newBr.Name:
		diffInstance = true
	}
	klog.V(4).Infof("UpdateSharedConfigMap key %s diff %v", share.Name, diffInstance)
	if !diffInstance {
		shareConfigMapsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		return
	}

	shareConfigMaps.Store(share.Name, share)
	br := share.Spec.ConfigMapRef
	key := BuildKey(br.Namespace, br.Name)
	configmapsWithShares.Delete(key)
	AddSharedConfigMap(share)
}

// UpdateSharedSecret updates the SharedSecret in our various tracking maps and if need be calls
// the registered callbacks to update the content on any volumes using the SharedSecret
func UpdateSharedSecret(share *sharev1alpha1.SharedSecret) {
	klog.V(4).Infof("UpdateSharedSecret key %s", share.Name)
	old, ok := shareSecrets.Load(share.Name)
	if !ok || old == nil {
		AddSharedSecret(share)
		return
	}
	oldShare := old.(*sharev1alpha1.SharedSecret)
	diffInstance := false
	oldBr := oldShare.Spec.SecretRef
	newBr := share.Spec.SecretRef
	switch {
	case oldBr.Namespace != newBr.Namespace:
		diffInstance = true
	case oldBr.Name != newBr.Name:
		diffInstance = true
	}
	klog.V(4).Infof("UpdateSharedSecret key %s diff %v", share.Name, diffInstance)
	if !diffInstance {
		shareSecretsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		return
	}

	shareSecrets.Store(share.Name, share)
	br := share.Spec.SecretRef
	key := BuildKey(br.Namespace, br.Name)
	secretsWithShare.Delete(key)
	AddSharedSecret(share)
}

// DelSharedConfigMap removes the SharedConfigMap from our various tracking maps and calls the registered callbacks
// to delete the config map content from any volumes using the SharedConfigMap
func DelSharedConfigMap(share *sharev1alpha1.SharedConfigMap) {
	br := share.Spec.ConfigMapRef
	key := BuildKey(br.Namespace, br.Name)
	klog.V(4).Infof("DelSharedConfigMap key %s", key)
	configmapsWithShares.Delete(key)
	shareConfigMaps.Delete(share.Name)
	shareConfigMapsDeleteCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
}

// DelSharedSecret removes the SharedSecret from our various tracking maps and calls the registered callbacks
// to delete the secret content from any volumes using the SharedSecret
func DelSharedSecret(share *sharev1alpha1.SharedSecret) {
	br := share.Spec.SecretRef
	key := BuildKey(br.Namespace, br.Name)
	klog.V(4).Infof("DelSharedSecret key %s", key)
	secretsWithShare.Delete(key)
	shareSecrets.Delete(share.Name)
	shareSecretsDeleteCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
}

// RegisterSharedConfigMapUpdateCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// then on controller update events for a SharedConfigMap, then function registered here will be called to possibly change
// storage
func RegisterSharedConfigMapUpdateCallback(volID string, f func(key, value interface{}) bool) {
	shareConfigMapsUpdateCallbacks.Store(volID, f)
	// cycle through the shareConfigMaps to find the one correlates to this volID's CSI volume mount request; the function
	// provided then completes the actual storage of the data in the pod
	shareConfigMaps.Range(f)
}

// RegisterSharedSecretUpdateCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// then on controller update events for a SharedSecret, then function registered here will be called to possibly change
// storage
func RegisterSharedSecretUpdateCallback(volID string, f func(key, value interface{}) bool) {
	shareSecretsUpdateCallbacks.Store(volID, f)
	shareSecrets.Range(f)
}

// UnregisterSharedConfigMapUpdateCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSharedConfigMapUpdateCallback(volID string) {
	configmapUpsertCallbacks.Delete(volID)
}

// UnregsiterSharedSecretsUpdateCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregsiterSharedSecretsUpdateCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

// RegisterSharedConfigMapDeleteCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// it records the CSI driver function to be called when a share is deleted, so that the CSI
// driver can remove any storage mounted in the pod for the given SharedConfigMap
func RegisterSharedConfigMapDeleteCallback(volID string, f func(key, value interface{}) bool) {
	shareConfigMapsDeleteCallbacks.Store(volID, f)
}

// RegisteredSharedSecretDeleteCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// it records the CSI driver function to be called when a share is deleted, so that the CSI
// driver can remove any storage mounted in the pod for the given SharedSecret
func RegisteredSharedSecretDeleteCallback(volID string, f func(key, value interface{}) bool) {
	shareSecretsDeleteCallbacks.Store(volID, f)
}

// UnregisterSharedConfigMapDeleteCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSharedConfigMapDeleteCallback(volID string) {
	shareConfigMapsDeleteCallbacks.Delete(volID)
}

// UnregisterSharedSecretDeleteCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSharedSecretDeleteCallback(volID string) {
	shareSecretsDeleteCallbacks.Delete(volID)
}
