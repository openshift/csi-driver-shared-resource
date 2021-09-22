package cache

import (
	"sync"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"

	sharev1alpha1 "github.com/openshift/csi-driver-shared-resource/pkg/api/sharedresource/v1alpha1"
)

/*
Some old fashioned comments that describe what we are doing in this golang file.

First, some notes on cardinality:
- 1 share at the moment only references 1 configmap or 1 secret
- 1 share can be used by many pods, and in particular, many CSI Driver volume mounts across those pods.  This driver
end of the day manages those CSI volumes, with of course the associated Pod and its various fields key metadata

Second, events.  As the AddShare, UpdateShare, or DelShare names imply, those methods are called when the controller
processes Add/Update/Delete events for SharedResource instances.  To date, the Update path is a superset of the Add path.  Or
in other words, the UpdateShare ultimately calls the AddShare function.

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

Fourth, a couple of notes on permissions and shares
- The SAR execution occurs on 2 events:
- 1) the pod creation / request to mount the CSI volume event from the kubelet
- 2) when a share update event occurs, which can be when the share is actually updated, or on the relist; the relist
variety is how we currently verify that a given pod is *STILL* allowed to access a given share if nothing else has changed.

*/

var (
	// shares is our global share ID (remember it is cluster scoped) to share map; generally, it facilitates lookup
	// when we are dealing with Pod creation events from the kubelet
	shares = sync.Map{}
	// sharesUpdateCallbacks have a key of the CSI volume ID and a value of the function to be called when a given
	// share is to updated, assuming the driver has mounted a share CSI volume in a pod somewhere.  Otherwise,
	// this map is empty and share updates result in a no-op.  This map is used both when we get an event for a given
	// share or a series of events as a result of a relist from the controller.
	shareUpdateCallbacks = sync.Map{}
	// same thing as shareUpdateCallbacks, but deletion of shares, and of of course the controller relist does not
	// come into play here.
	shareDeleteCallbacks = sync.Map{}
)

func AddShare(share *sharev1alpha1.SharedResource) {
	br := share.Spec.Resource
	key := BuildKey(share.Spec.Resource)
	klog.V(4).Infof("AddShare key %s kind %s", key, br.Type)
	switch br.Type {
	case sharev1alpha1.ResourceReferenceTypeConfigMap:
		obj, ok := configmaps.Load(key)
		if obj != nil && ok {
			cm := obj.(*corev1.ConfigMap)
			configmapsWithShares.Store(key, cm)
			// so this line build a map with a single entry, the share from this event, and then
			// applies the function(s) supplied by the CSI volume code in order to make changes based
			// on this event
			shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
			//NOTE we do not store share in shares unless the backing resource is available
			shares.Store(share.Name, share)
		} else {
			sharesWaitingOnConfigmaps.Store(share.Name, share)
		}
	case sharev1alpha1.ResourceReferenceTypeSecret:
		obj, ok := secrets.Load(key)
		if obj != nil && ok {
			s := obj.(*corev1.Secret)
			secretsWithShare.Store(key, s)
			// so this line build a map with a single entry, the share from this event, and then
			// applies the function(s) supplied by the CSI volume code in order to make changes based
			// on this event
			shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
			//NOTE we do not store share in shares unless the backing resource is available
			shares.Store(share.Name, share)
		} else {
			sharesWaitingOnSecrets.Store(share.Name, share)
		}
	}
}

func UpdateShare(share *sharev1alpha1.SharedResource) {
	klog.V(4).Infof("UpdateShare key %s kind %s", share.Name, share.Spec.Resource.Type)
	old, ok := shares.Load(share.Name)
	if !ok || old == nil {
		AddShare(share)
		return
	}
	oldShare := old.(*sharev1alpha1.SharedResource)
	diffInstance := false
	oldBr := oldShare.Spec.Resource
	newBr := share.Spec.Resource
	switch {
	case oldBr.Type != newBr.Type:
		diffInstance = true
	case GetResourceNamespace(oldBr) != GetResourceNamespace(newBr):
		diffInstance = true
	case GetResourceName(oldBr) != GetResourceName(newBr):
		diffInstance = true
	}
	klog.V(4).Infof("UpdateShare key %s kind %s diff %v", share.Name, share.Spec.Resource.Type, diffInstance)
	if !diffInstance {
		shareUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		return
	}

	shares.Store(share.Name, share)
	br := share.Spec.Resource
	key := BuildKey(br)
	configmapsWithShares.Delete(key)
	secretsWithShare.Delete(key)
	AddShare(share)
}

func DelShare(share *sharev1alpha1.SharedResource) {
	br := share.Spec.Resource
	key := BuildKey(br)
	klog.V(4).Infof("DelShare key %s kind %s", key, br.Type)
	configmapsWithShares.Delete(key)
	secretsWithShare.Delete(key)
	shares.Delete(share.Name)
	shareDeleteCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
}

// RegisterShareUpdateCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// then on controller update events for a share, then function registered here will be called to possibly change
// storage
func RegisterShareUpdateCallback(volID string, f func(key, value interface{}) bool) {
	shareUpdateCallbacks.Store(volID, f)
	// cycle through the shares to find the one correlates to this volID's CSI volume mount request; the function
	// provided then completes the actual storage of the data in the pod
	shares.Range(f)
}

// UnregisterShareUpdateCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterShareUpdateCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

// RegisterShareDeleteCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// it records the CSI driver function to be called when a share is deleted, so that the CSI
// driver can remove any storage mounted in the pod for the given share
func RegisterShareDeleteCallback(volID string, f func(key, value interface{}) bool) {
	shareDeleteCallbacks.Store(volID, f)
}

// UnregisterShareDeleteCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterShareDeleteCallback(volID string) {
	shareDeleteCallbacks.Delete(volID)
}
