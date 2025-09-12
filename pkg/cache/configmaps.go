package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/config"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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
	// configmapUpsertCallbacks has a key of the CSI volume ID and a value of the function to be called when a given
	// configmap is updated, assuming the driver has mounted a share CSI volume with the configmap in a pod somewhere, and
	// the corresponding storage on the pod gets updated by the function that is the value of the entry.  Otherwise,
	// this map is empty and configmap updates result in a no-op.  This map is used both when we get an event for a given
	// configmap or a series of events as a result of a relist from the controller.
	configmapUpsertCallbacks = sync.Map{}
	// same thing as configmapUpsertCallbacks, but deletion of configmaps, and of of course the controller relist does not
	// come into play here.
	configmapDeleteCallbacks = sync.Map{}
)

// UpsertConfigMap adds or updates as needed the config map to our various maps for correlating with SharedConfigMaps and
// calls registered upsert callbacks
func UpsertConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	klog.V(6).Infof("UpsertConfigMap key %s", key)
	// first, find the shares pointing to this configmap, and call the callbacks, in case certain pods
	// have had their permissions revoked; this will also handle if we had share events arrive before
	// the corresponding configmap
	sharecConfigMapList := client.ListSharedConfigMap()
	for _, share := range sharecConfigMapList {
		if share.Spec.ConfigMapRef.Namespace == configmap.Namespace && share.Spec.ConfigMapRef.Name == configmap.Name {
			shareConfigMapsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		}
	}
	// otherwise process any share that arrived after the configmap
	configmapUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, configmap)))
}

// DelConfigMap deletes this config map from the various configmap related maps
func DelConfigMap(configmap *corev1.ConfigMap) {
	key := GetKey(configmap)
	klog.V(4).Infof("DelConfigMap key %s", key)
	configmapDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, configmap)))
}

// RegisterConfigMapUpsertCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// if the corresponding share references a configmap, then the function registered here will be called to possibly change
// storage
func RegisterConfigMapUpsertCallback(volID, cmID string, f func(key, value interface{}) bool) error{
	if !config.LoadedConfig.RefreshResources {
		return nil
	}
	configmapUpsertCallbacks.Store(volID, f)
	ns, name, _ := SplitKey(cmID)
	cm, err := client.GetConfigMap(ns, name)
	if err != nil {
		klog.Warningf("could not get configmap for %s vol %s", cmID, volID)
		
		if kerrors.IsForbidden(err) {
			return status.Errorf(codes.PermissionDenied, "csi driver is forbidden to access configmap %s/%s: %v", ns, name, err)
		}
		return status.Errorf(codes.Internal, "csi driver failed to get configmap %s/%s: %v", ns, name, err)
	}

	if cm != nil {
		f(BuildKey(cm.Namespace, cm.Name), cm)
	} else {
		klog.Warningf("not found on get configmap for %s vol %s", cmID, volID)
	}
	return nil
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
