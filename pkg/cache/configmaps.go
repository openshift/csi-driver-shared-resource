package cache

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
)

/*
Some old fashioned comments that describe what we are doing in this golang file.

First, some notes on cardinality:
- 1 share references 1 and only 1 configmap currently
- given share related cardinality, many pods can reference a given configmap via share CSI Volumes

Second, events.  We process Add and Update configmap events from the controller in the same way, so we have an UpsertConfigMap function.
For delete events, the DelConfigMap is called.

On the use of sync.Map, see the comments in share.go
*/

var (
	// configmaps is our global configmap id (namespace + name) to configmap map, where entries are populated from
	// controller events; it serves to facilitate quick lookup during share event processing, when the share references
	// a configmap
	configmaps = sync.Map{}
	// configmapUpsertCallbacks has a key of the CSI volume ID and a value of the function to be called when a given
	// configmap is updated, assuming the driver has mounted a share CSI volume with the configmap in a pod somewhere, and
	// the corresponding storage on the pod gets updated by the function that is the value of the entry.  Otherwise,
	// this map is empty and configmap updates result in a no-op.  This map is used both when we get an event for a given
	// configmap or a series of events as a result of a relist from the controller.
	configmapUpsertCallbacks = sync.Map{}
	// same thing as configmapUpsertCallbacks, but deletion of configmaps, and of of course the controller relist does not
	// come into play here.
	configmapDeleteCallbacks = sync.Map{}
	// configmapsWithShares is a filtered list of configmaps where, via share events, we know at least one active share references
	// a given configmap; when possible we range over this list vs. configsmaps
	configmapsWithShares = sync.Map{}
	// sharesWaitingOnConfigmaps conversely is for when a share has been created that references a configmap, but that
	// configmap has not been recognized by the controller; quite possibly timing events on when we learn of sharedConfigMaps
	// and configmaps if they happen to be created at roughly the same time come into play; also, if a pod with a share
	// pointing to a configmap has been provisioned, but the the csi driver daemonset has been restarted, such timing
	// of events where we learn of sharesConfigMaps before their configmaps can also occur, as we attempt to rebuild the CSI driver's
	// state
	sharesWaitingOnConfigmaps = sync.Map{}
)

// GetConfigMap retrieves a config map from the list of config maps referenced by SharedConfigMaps
func GetConfigMap(key interface{}) *corev1.ConfigMap {
	obj, loaded := configmapsWithShares.Load(key)
	if loaded {
		cm, _ := obj.(*corev1.ConfigMap)
		return cm
	}
	return nil
}

// SetConfigMap based on the shared-data-key, which contains the resource's namespace and name, this
// method can fetch and store it on cache.  This method is called when the controller is not watching
// config maps, and the CSI driver must retrieve the config map when processing a NodePublishVolume call
// from the kubelet.
func SetConfigMap(kubeClient kubernetes.Interface, sharedDataKey string) error {
	ns, name, err := SplitKey(sharedDataKey)
	if err != nil {
		return err
	}

	cm, err := kubeClient.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	UpsertConfigMap(cm)
	return nil
}

// UpsertConfigMap adds or updates as needed the config map to our various maps for correlating with SharedConfigMaps and
// calls registered upsert callbacks
func UpsertConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	klog.V(6).Infof("UpsertConfigMap key %s", key)
	configmaps.Store(key, configmap)
	// in case share arrived before configmap
	processSharesWithoutConfigmaps := []string{}
	sharesWaitingOnConfigmaps.Range(func(key, value interface{}) bool {
		shareKey := key.(string)
		share := value.(*sharev1alpha1.SharedConfigMap)
		br := share.Spec.ConfigMapRef
		configmapKey := BuildKey(br.Namespace, br.Name)
		configmapsWithShares.Store(configmapKey, configmap)
		//NOTE: share update ranger will store share in sharedConfigMaps sync.Map
		// and we are supplying only this specific share to the csi driver update range callbacks.
		shareConfigMapsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		processSharesWithoutConfigmaps = append(processSharesWithoutConfigmaps, shareKey)
		return true
	})
	for _, shareKey := range processSharesWithoutConfigmaps {
		sharesWaitingOnConfigmaps.Delete(shareKey)
	}
	// otherwise process any share that arrived after the configmap
	configmapsWithShares.Store(key, configmap)
	configmapUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, configmap)))
}

// DelConfigMap deletes this config map from the various configmap related maps
func DelConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	klog.V(4).Infof("DelConfigMap key %s", key)
	configmaps.Delete(key)
	configmapDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, configmap)))
	configmapsWithShares.Delete(key)
}

// RegisterConfigMapUpsertCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// if the corresponding share references a configmap, then the function registered here will be called to possibly change
// storage
func RegisterConfigMapUpsertCallback(volID string, f func(key, value interface{}) bool) {
	configmapUpsertCallbacks.Store(volID, f)
	// cycle through the configmaps with sharedConfigMaps, where if the share associated with the volID CSI volume mount references
	// one of the configmaps provided by the Range, the storage of the corresponding data on the pod will be completed using
	// the supplied function
	configmapsWithShares.Range(f)
}

// UnregisterConfigMapUpsertCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterConfigMapUpsertCallback(volID string) {
	configmapUpsertCallbacks.Delete(volID)
}

// RegisterConfigMapDeleteCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// it records the CSI driver function to be called when a configmap is deleted, so that the CSI
// driver can remove any storage mounted in the pod for the given configmap
func RegisterConfigMapDeleteCallback(volID string, f func(key, value interface{}) bool) {
	configmapDeleteCallbacks.Store(volID, f)
}

// UnregisterConfigMapDeleteCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterConfigMapDeleteCallback(volID string) {
	configmapDeleteCallbacks.Delete(volID)
}
