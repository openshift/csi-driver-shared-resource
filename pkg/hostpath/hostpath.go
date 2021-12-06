/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hostpath

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	objcache "github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/config"
	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
)

type hostPath struct {
	name              string
	nodeID            string
	version           string
	endpoint          string
	ephemeral         bool
	maxVolumesPerNode int64

	ids *identityServer
	ns  *nodeServer

	root string
}

var (
	vendorVersion = "dev"

	hostPathVolumes = sync.Map{}
)

const (
	// Directory where data for volumes are persisted.
	// This is ephemeral to facilitate our per-pod, tmpfs,
	// no bind mount, approach.
	DataRoot = "/run/csi-data-dir"

	// Directory where we persist `hostPathVolumes`
	// This is a hostpath volume on the local node
	// to maintain state across restarts of the DaemonSet
	VolumeMapRoot = "/csi-volumes-map"
)

func (hp *hostPath) getHostpathVolume(name string) *hostPathVolume {
	obj, loaded := hostPathVolumes.Load(name)
	if loaded {
		hpv, _ := obj.(*hostPathVolume)
		return hpv
	}
	return nil
}

func setHPV(name string, hpv *hostPathVolume) {
	if hpv.Lock == nil {
		hpv.Lock = &sync.Mutex{}
	}
	hostPathVolumes.Store(name, hpv)
}

func remHPV(name string) {
	hostPathVolumes.Delete(name)
}

type HostPathDriver interface {
	createHostpathVolume(volID, targetPath string, readOnly, refresh bool, volCtx map[string]string, cmShare *sharev1alpha1.SharedConfigMap, sShare *sharev1alpha1.SharedSecret, cap int64, volAccessType accessType) (*hostPathVolume, error)
	getHostpathVolume(volID string) *hostPathVolume
	deleteHostpathVolume(volID string) error
	getVolumePath(volID string, volCtx map[string]string) (string, string)
	mapVolumeToPod(hpv *hostPathVolume) error
	Run()
	GetRoot() string
}

// NewHostPathDriver instantiate the HostPathDriver with the driver details.  Optionally, a
// Kubernetes Clientset can be informed to update (warm up) the object cache before creating the
// volume (and it's data) for mounting on the incoming pod.
func NewHostPathDriver(root, volMapRoot, driverName, nodeID, endpoint string, maxVolumesPerNode int64, version string) (HostPathDriver, error) {
	if driverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if nodeID == "" {
		return nil, errors.New("no node id provided")
	}

	if endpoint == "" {
		return nil, errors.New("no driver endpoint provided")
	}
	if version != "" {
		vendorVersion = version
	}

	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, fmt.Errorf("failed to create DataRoot: %v", err)
	}

	if err := os.MkdirAll(volMapRoot, 0750); err != nil {
		return nil, fmt.Errorf("failed to create VolMapRoot: %v", err)
	}

	klog.Infof("Driver: '%v', Version: '%s'", driverName, vendorVersion)
	klog.Infof("EndPoint: '%s', NodeID: '%s'", endpoint, nodeID)

	if !config.LoadedConfig.RefreshResources {
		klog.Info("RefreshResources is disabled and HostPathDriver will directly read Kubernetes corev1 resources!")
	}

	hp := &hostPath{
		name:              driverName,
		version:           vendorVersion,
		nodeID:            nodeID,
		endpoint:          endpoint,
		maxVolumesPerNode: maxVolumesPerNode,
		root:              root,
	}

	if err := hp.loadVolsFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load volume map on disk: %v", err)
	}

	return hp, nil
}

func (hp *hostPath) GetRoot() string {
	return hp.root
}

func (hp *hostPath) Run() {
	// Create GRPC servers
	hp.ids = NewIdentityServer(hp.name, hp.version)

	// the node-server will be on always-read-only mode when the object-cache is being populated
	// directly
	alwaysReadOnly := !config.LoadedConfig.RefreshResources
	hp.ns = NewNodeServer(hp, alwaysReadOnly)

	s := NewNonBlockingGRPCServer()
	s.Start(hp.endpoint, hp.ids, hp.ns)
	s.Wait()
}

// getVolumePath returns the canonical paths for hostpath volume
func (hp *hostPath) getVolumePath(volID string, volCtx map[string]string) (string, string) {
	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	mountIDString := strings.Join([]string{podNamespace, podName, volID}, "-")
	return mountIDString, filepath.Join(hp.root, bindDir, volID, podNamespace, podName, podUID, podSA)
}

func commonRangerProceedFilter(hpv *hostPathVolume, key interface{}) bool {
	if hpv == nil {
		return false
	}
	compareKey := ""
	// see if the shared item pertains to this volume
	switch hpv.GetSharedDataKind() {
	case consts.ResourceReferenceTypeSecret:
		sharedSecret := client.GetSharedSecret(hpv.GetSharedDataId())
		if sharedSecret == nil {
			klog.V(6).Infof("commonRangerProceedFilter could not retrieve share %s for %s:%s:%s", hpv.GetSharedDataId(), hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetVolID())
			return false
		}
		compareKey = objcache.BuildKey(sharedSecret.Spec.SecretRef.Namespace, sharedSecret.Spec.SecretRef.Name)
	case consts.ResourceReferenceTypeConfigMap:
		sharedConfigMap := client.GetSharedConfigMap(hpv.GetSharedDataId())
		if sharedConfigMap == nil {
			klog.V(6).Infof("commonRangerProceedFilter could not retrieve share %s for %s:%s:%s", hpv.GetSharedDataId(), hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetVolID())
			return false
		}
		compareKey = objcache.BuildKey(sharedConfigMap.Spec.ConfigMapRef.Namespace, sharedConfigMap.Spec.ConfigMapRef.Name)
	default:
		klog.Warningf("commonRangerProceedFilter unknown share type for %s:%s:%s: %s", hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetVolID())
		return false
	}
	keyStr := key.(string)
	if keyStr != compareKey {
		klog.V(4).Infof("commonRangerProceedFilter skipping %s as it does not match %s for %s:%s:%s", keyStr, compareKey, hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetVolID())
		return false
	}
	return true
}

func commonUpsertRanger(hpv *hostPathVolume, key, value interface{}) error {
	proceed := commonRangerProceedFilter(hpv, key)
	if !proceed {
		return nil
	}

	payload, _ := value.(Payload)
	klog.V(4).Infof("commonUpsertRanger key %s hpv %#v", key, hpv)
	podPath := hpv.GetTargetPath()
	if hpv.IsReadOnly() {
		podPath = hpv.GetVolPathBindMountDir()
	}
	// So, what to do with error handling.  Errors with filesystem operations
	// will almost always not be intermittent, but most likely the result of the
	// host filesystem either being full or compromised in some long running fashion, so tight-loop retry, like we
	// *could* do here as a result will typically prove fruitless.
	// Then, the controller relist will result in going through the secrets/configmaps we share, so
	// again, on the off chance the filesystem error is intermittent, or if an administrator has taken corrective
	// action, writing the content will be retried.  And note, the relist interval is configurable (default 10 minutes)
	// if users want more rapid retry...but by default, no tight loop more CPU intensive retry
	// Lastly, with the understanding that an error log in the pod stdout may be missed, we will also generate a k8s
	// event to facilitate exposure
	// TODO: prometheus metrics/alerts may be desired here, though some due diligence on what k8s level metrics/alerts
	// around host filesystem issues might already exist would be warranted with such an exploration/effort

	// Next, on an update we first nuke any existing directory and then recreate it to simplify handling the case where
	// the keys in the secret/configmap have changed such that some keys have been removed, which would translate
	// in files having to be removed. commonOSRemove will handle shares mounted off of shares.  And a reminder,
	// currently this driver does not support overlaying over directories with files.  Either the directory in the
	// container image must be empty, or the directory does not exist, and is created for the Pod's container as
	// part of provisioning the container.
	if err := commonOSRemove(podPath, fmt.Sprintf("commonUpsertRanger key %s volid %s share id %s pod name %s", key, hpv.GetVolID(), hpv.GetSharedDataId(), hpv.GetPodName())); err != nil {
		return err
	}
	if err := os.MkdirAll(podPath, os.ModePerm); err != nil {
		return err
	}
	if payload.ByteData != nil {
		for dataKey, dataValue := range payload.ByteData {
			podFilePath := filepath.Join(podPath, dataKey)
			klog.V(4).Infof("commonUpsertRanger create/update file %s key %s volid %s share id %s pod name %s", podFilePath, key, hpv.GetVolID(), hpv.GetSharedDataId(), hpv.GetPodName())
			if err := ioutil.WriteFile(podFilePath, dataValue, 0644); err != nil {
				return err
			}

		}
	}
	if payload.StringData != nil {
		for dataKey, dataValue := range payload.StringData {
			podFilePath := filepath.Join(podPath, dataKey)
			klog.V(4).Infof("commonUpsertRanger create/update file %s key %s volid %s share id %s pod name %s", podFilePath, key, hpv.GetVolID(), hpv.GetSharedDataId(), hpv.GetPodName())
			content := []byte(dataValue)
			if err := ioutil.WriteFile(podFilePath, content, 0644); err != nil {
				return err
			}
		}
	}
	klog.V(4).Infof("common upsert ranger returning key %s", key)
	return nil
}

func commonOSRemove(dir, dbg string) error {
	klog.V(4).Infof("commonOSRemove to delete %q dbg %s", dir, dbg)
	defer klog.V(4).Infof("commonOSRemove completed delete attempt for dir %q", dir)
	// we cannot do a os.RemoveAll on the mount point, so we remove all on each file system entity
	// off of the potential mount point
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		// since we do not support mounting on existing content, a dir can only mean a share
		// has been mounted as a separate dir in our share, so skip
		if info.IsDir() {
			return nil
		}
		fileName := filepath.Join(dir, info.Name())
		klog.V(4).Infof("commonOSRemove going to delete file %s", fileName)
		return os.RemoveAll(fileName)
	})

}

func commonDeleteRanger(hpv *hostPathVolume, key interface{}) bool {
	proceed := commonRangerProceedFilter(hpv, key)
	if !proceed {
		// even though we are aborting, return true to continue to next entry in ranger list
		return true
	}
	podPath := hpv.GetTargetPath()
	if hpv.IsReadOnly() {
		podPath = hpv.GetVolPathBindMountDir()
	}
	klog.V(4).Infof("common delete ranger key %s", key)
	commonOSRemove(podPath, fmt.Sprintf("commonDeleteRanger %s", key))
	klog.V(4).Infof("common delete ranger returning key %s", key)
	return true
}

type innerShareDeleteRanger struct {
	shareId string
}

func (r *innerShareDeleteRanger) Range(key, value interface{}) bool {
	targetPath := ""
	volID := key.(string)
	// painful debug has shown you cannot trust the value that comes in, you have to refetch,
	// unless the map only has 1 entry in it
	var hpv *hostPathVolume
	klog.V(4).Infof("innerShareDeleteRanger key %q\n incoming share id %s",
		key,
		r.shareId)
	hpvObj, ok := hostPathVolumes.Load(key)
	if !ok {
		klog.V(0).Infof("innerShareDeleteRanger how the hell can we not load key %s from the range list", key)
		// continue to the next entry, skip this one
		return true
	} else {
		hpv, _ = hpvObj.(*hostPathVolume)
	}
	if hpv.GetVolID() == volID && hpv.GetSharedDataId() == r.shareId {
		klog.V(4).Infof("innerShareDeleteRanger shareid %s kind %s", r.shareId, hpv.GetSharedDataKind())
		if hpv.IsReadOnly() {
			targetPath = hpv.GetVolPathBindMountDir()

		} else {
			targetPath = hpv.GetTargetPath()
		}
		volID = hpv.GetVolID()
		if len(volID) > 0 && len(targetPath) > 0 {
			err := commonOSRemove(targetPath, fmt.Sprintf("innerShareDeleteRanger shareID id %s", r.shareId))
			if err != nil {
				klog.Warningf("innerShareDeleteRanger %s vol %s target path %s delete error %s",
					r.shareId, volID, targetPath, err.Error())
			}
			// we just delete the associated data from the previously provisioned volume;
			// we don't delete the volume in case the share is added back
		}
		return false
	}
	return true
}

func shareDeleteRanger(key interface{}) bool {
	shareId := key.(string)
	klog.V(4).Infof("shareDeleteRanger shareID id %s", shareId)
	ranger := &innerShareDeleteRanger{
		shareId: shareId,
	}

	hostPathVolumes.Range(ranger.Range)
	klog.V(4).Infof("shareDeleteRanger returning share id %s", shareId)
	return true
}

type innerShareUpdateRanger struct {
	shareId   string
	secret    bool
	configmap bool

	oldTargetPath string
	sharedItemKey string
	volID         string

	sharedItem Payload
}

func (r *innerShareUpdateRanger) Range(key, value interface{}) bool {
	volID := key.(string)
	// painful debug has shown you cannot trust the value that comes in, you have to refetch,
	// unless the map only has 1 entry in it
	var hpv *hostPathVolume
	klog.V(4).Infof("innerShareUpdateRanger key %q\n incoming share id %s",
		key,
		r.shareId)
	hpvObj, ok := hostPathVolumes.Load(key)
	if !ok {
		klog.V(0).Infof("innerShareUpdateRanger how the hell can we not load key %s from the range list", key)
		// continue to the next entry, skip this one
		return true
	} else {
		hpv, _ = hpvObj.(*hostPathVolume)
	}
	if hpv.GetVolID() == volID && hpv.GetSharedDataId() == r.shareId {
		klog.V(4).Infof("innerShareUpdateRanger MATCH inner ranger key %q\n hpv vol id %s\n incoming share id %s\n hpv share id %s", key, hpv.GetVolID(), r.shareId, hpv.GetSharedDataId())
		a, err := client.ExecuteSAR(r.shareId, hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetPodSA(), hpv.GetSharedDataKind())
		allowed := a && err == nil

		if allowed {
			klog.V(0).Infof("innerShareUpdateRanger pod %s:%s has permissions for secretShare %s",
				hpv.GetPodNamespace(), hpv.GetPodName(), r.shareId)
		} else {
			klog.V(0).Infof("innerShareUpdateRanger pod %s:%s does not permission for secretShare %s",
				hpv.GetPodNamespace(), hpv.GetPodName(), r.shareId)
		}

		switch {
		case r.secret:
			sharedSecret := client.GetSharedSecret(r.shareId)
			if sharedSecret == nil {
				klog.Warningf("innerShareUpdateRanger unexpected not found on sharedSecret lister refresh: %s", r.shareId)
				return false
			}
			r.sharedItemKey = objcache.BuildKey(sharedSecret.Spec.SecretRef.Namespace, sharedSecret.Spec.SecretRef.Name)
			secretObj := client.GetSecret(sharedSecret.Spec.SecretRef.Namespace, sharedSecret.Spec.SecretRef.Name)
			if secretObj == nil {
				klog.Infof("innerShareUpdateRanger share %s could not retrieve shared item %s", r.shareId, r.sharedItemKey)
				return false
			}
			r.sharedItem = Payload{
				ByteData:   secretObj.Data,
				StringData: secretObj.StringData,
			}
		case r.configmap:
			sharedConfigMap := client.GetSharedConfigMap(r.shareId)
			if sharedConfigMap == nil {
				klog.Warningf("innerShareUpdateRanger unexpected not found on sharedConfigMap lister refresh: %s", r.shareId)
				return false
			}
			r.sharedItemKey = objcache.BuildKey(sharedConfigMap.Spec.ConfigMapRef.Namespace, sharedConfigMap.Spec.ConfigMapRef.Name)
			cmObj := client.GetConfigMap(sharedConfigMap.Spec.ConfigMapRef.Namespace, sharedConfigMap.Spec.ConfigMapRef.Name)
			if cmObj == nil {
				klog.Infof("innerShareUpdateRanger share %s could not retrieve shared item %s", r.shareId, r.sharedItemKey)
				return false
			}
			r.sharedItem = Payload{
				StringData: cmObj.Data,
				ByteData:   cmObj.BinaryData,
			}
		}

		if hpv.IsReadOnly() {
			r.oldTargetPath = hpv.GetVolPathBindMountDir()
		} else {
			r.oldTargetPath = hpv.GetTargetPath()
		}
		r.volID = hpv.GetVolID()

		if !allowed {
			err := commonOSRemove(r.oldTargetPath, "lostPermissions")
			if err != nil {
				klog.Warningf("innerShareUpdateRanger %s target path %s delete error %s",
					key, r.oldTargetPath, err.Error())
			}
			//TODO removing contents from a read only volume, where we employ an intermediate bind mount, is the single
			// item in our list of update content features that still works when this driver is restarted after a Pod
			// is started with one of our volumes.  The question is do we even bother supporting this, or just make
			// the general statement that "results may vary" and do not claim any production level support for updating
			// contents with read only volumes since we don't have a comprehensive solution for when the driver is restarted.
			if hpv.IsReadOnly() {
				tp := hpv.GetTargetPath()
				empty, err := isDirEmpty(tp)
				errStr := ""
				if err != nil {
					errStr = err.Error()
				}
				klog.V(4).Infof("innerShareUpdateRanger kubelet dir %s empty %v err %s", tp, empty, errStr)
				if !empty {
					err = commonOSRemove(tp, "lostPermissions")
					if err != nil {
						errStr = err.Error()
					}
					klog.V(4).Infof("innerShareUpdateRanger kubelet dir %s commonOsRemove err %s", tp, errStr)

				}
			}
			objcache.UnregisterSecretUpsertCallback(r.volID)
			objcache.UnregisterSecretDeleteCallback(r.volID)
			objcache.UnregisterConfigMapDeleteCallback(r.volID)
			objcache.UnregisterConfigMapUpsertCallback(r.volID)
			return false
		}

		commonUpsertRanger(hpv, r.sharedItemKey, r.sharedItem)

	}
	klog.V(4).Infof("innerShareUpdateRanger NO MATCH inner ranger key %q\n hpv vol id %s\n incoming share id %s\n hpv share id %s", key, hpv.GetVolID(), r.shareId, hpv.GetSharedDataId())
	return true
}

func shareUpdateRanger(key, value interface{}) bool {
	shareId := key.(string)
	_, sok := value.(*sharev1alpha1.SharedSecret)
	_, cmok := value.(*sharev1alpha1.SharedConfigMap)
	if !sok && !cmok {
		klog.Warningf("unknown shareUpdateRanger key %q object %#v", key, value)
		return false
	}
	klog.V(4).Infof("shareUpdateRanger key %s secret %v configmap %v", key, sok, cmok)
	rangerObj := &innerShareUpdateRanger{
		shareId:   shareId,
		secret:    sok,
		configmap: cmok,
	}
	hostPathVolumes.Range(rangerObj.Range)

	klog.V(4).Infof("shareUpdateRanger key %s value %#v inner ranger %#v inner ranger", key, value, rangerObj)
	return true
}

func mapBackingResourceToPod(hpv *hostPathVolume) error {
	klog.V(4).Infof("mapBackingResourceToPod")
	readOnly := hpv.IsReadOnly()
	// for now, since os.MkdirAll does nothing and returns no error when the path already
	// exists, we have a common path for both create and update; but if we change the file
	// system interaction mechanism such that create and update are treated differently, we'll
	// need separate callbacks for each
	if readOnly {
		err := os.MkdirAll(hpv.GetVolPathBindMountDir(), 0777)
		if err != nil {
			return err
		}
	}
	switch hpv.GetSharedDataKind() {
	case consts.ResourceReferenceTypeConfigMap:
		klog.V(4).Infof("mapBackingResourceToPod postlock %s configmap", hpv.GetVolID())
		upsertRangerCM := func(key, value interface{}) bool {
			cm, _ := value.(*corev1.ConfigMap)
			payload := Payload{
				StringData: cm.Data,
				ByteData:   cm.BinaryData,
			}
			err := commonUpsertRanger(hpv, key, payload)
			if err != nil {
				ProcessFileSystemError(cm, err)
			}

			// we always return true in the golang ranger to still attempt additional items
			// on the off chance the filesystem error received was intermittent and other items
			// will succeed ... remember, the ranger predominantly deals with pushing secret/configmap
			// updates to disk
			return true
		}
		// we call the upsert ranger inline in case there are filesystem problems initially, so
		// we can return the error back to volume provisioning, where the kubelet will retry at
		// a controlled frequency
		sharedConfigMap := client.GetSharedConfigMap(hpv.GetSharedDataId())
		if sharedConfigMap == nil {
			klog.V(4).Infof("mapBackingResourceToPod for pod volume %s:%s:%s share %s no longer exists", hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetVolID(), hpv.GetSharedDataId())
			return nil
		}
		cmNamespace := sharedConfigMap.Spec.ConfigMapRef.Namespace
		cmName := sharedConfigMap.Spec.ConfigMapRef.Name
		comboKey := objcache.BuildKey(cmNamespace, cmName)
		cm := client.GetConfigMap(cmNamespace, cmName)
		if cm != nil {
			payload := Payload{
				StringData: cm.Data,
				ByteData:   cm.BinaryData,
			}

			upsertError := commonUpsertRanger(hpv, comboKey, payload)
			if upsertError != nil {
				ProcessFileSystemError(cm, upsertError)
				return upsertError
			}
		}
		if hpv.IsRefresh() {
			objcache.RegisterConfigMapUpsertCallback(hpv.GetVolID(), comboKey, upsertRangerCM)
		}
		deleteRangerCM := func(key, value interface{}) bool {
			return commonDeleteRanger(hpv, key)
		}
		//we should register delete callbacks regardless of any per volume refresh setting to account for removed permissions
		objcache.RegisterConfigMapDeleteCallback(hpv.GetVolID(), deleteRangerCM)
	case consts.ResourceReferenceTypeSecret:
		klog.V(4).Infof("mapBackingResourceToPod postlock %s secret", hpv.GetVolID())
		upsertRangerSec := func(key, value interface{}) bool {
			s, _ := value.(*corev1.Secret)
			payload := Payload{
				ByteData: s.Data,
			}
			err := commonUpsertRanger(hpv, key, payload)
			if err != nil {
				ProcessFileSystemError(s, err)
			}
			// we always return true in the golang ranger to still attempt additional items
			// on the off chance the filesystem error received was intermittent and other items
			// will succeed ... remember, the ranger predominantly deals with pushing secret/configmap
			// updates to disk
			return true
		}
		// we call the upsert ranger inline in case there are filesystem problems initially,  so
		// we can return the error back to volume provisioning, where the kubelet will retry at
		// a controlled frequency
		sharedSecret := client.GetSharedSecret(hpv.GetSharedDataId())
		sNamespace := sharedSecret.Spec.SecretRef.Namespace
		sName := sharedSecret.Spec.SecretRef.Name
		comboKey := objcache.BuildKey(sNamespace, sName)
		s := client.GetSecret(sNamespace, sName)
		if s != nil {
			payload := Payload{
				ByteData: s.Data,
			}

			upsertError := commonUpsertRanger(hpv, comboKey, payload)
			if upsertError != nil {
				ProcessFileSystemError(s, upsertError)
				return upsertError
			}
		}
		if hpv.IsRefresh() {
			objcache.RegisterSecretUpsertCallback(hpv.GetVolID(), comboKey, upsertRangerSec)
		}
		deleteRangerSec := func(key, value interface{}) bool {
			return commonDeleteRanger(hpv, key)
		}
		//we should register delete callbacks regardless of any per volume refresh setting to account for removed permissions
		objcache.RegisterSecretDeleteCallback(hpv.GetVolID(), deleteRangerSec)
	default:
		return fmt.Errorf("invalid share backing resource kind %s", hpv.GetSharedDataKind())
	}
	return nil
}

func (hp *hostPath) mapVolumeToPod(hpv *hostPathVolume) error {
	klog.V(4).Infof("mapVolumeToPod calling mapBackingResourceToPod")

	err := mapBackingResourceToPod(hpv)
	if err != nil {
		return err
	}
	hp.registerRangers(hpv)

	return nil
}

func (hp *hostPath) registerRangers(hpv *hostPathVolume) {
	deleteRangerShare := func(key, value interface{}) bool {
		return shareDeleteRanger(key)
	}
	updateRangerShare := func(key, value interface{}) bool {
		return shareUpdateRanger(key, value)
	}
	switch hpv.GetSharedDataKind() {
	case consts.ResourceReferenceTypeSecret:
		objcache.RegisterSharedSecretUpdateCallback(hpv.GetVolID(), hpv.GetSharedDataId(), updateRangerShare)
		objcache.RegisteredSharedSecretDeleteCallback(hpv.GetVolID(), deleteRangerShare)
	case consts.ResourceReferenceTypeConfigMap:
		objcache.RegisterSharedConfigMapUpdateCallback(hpv.GetVolID(), hpv.GetSharedDataId(), updateRangerShare)
		objcache.RegisterSharedConfigMapDeleteCallback(hpv.GetVolID(), deleteRangerShare)
	}

}

// createVolume create the directory for the hostpath volume.
// It returns the volume path or err if one occurs.
func (hp *hostPath) createHostpathVolume(volID, targetPath string, readOnly, refresh bool, volCtx map[string]string, cmShare *sharev1alpha1.SharedConfigMap, sShare *sharev1alpha1.SharedSecret, cap int64, volAccessType accessType) (*hostPathVolume, error) {
	if cmShare != nil && sShare != nil {
		return nil, fmt.Errorf("cannot store both SharedConfigMap and SharedSecret in a volume")
	}
	if cmShare == nil && sShare == nil {
		return nil, fmt.Errorf("have to provide either a SharedConfigMap or SharedSecret to a volume")
	}
	hpv := hp.getHostpathVolume(volID)
	if hpv != nil {
		klog.V(0).Infof("createHostpathVolume: create call came in for volume %s that we have already created; returning previously created instance", volID)
		return hpv, nil
	}
	anchorDir, bindDir := hp.getVolumePath(volID, volCtx)
	switch volAccessType {
	case mountAccess:
		if readOnly {
			err := os.MkdirAll(bindDir, 0777)
			if err != nil {
				return nil, err
			}
		}
		err := os.MkdirAll(anchorDir, 0777)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	hostpathVol := CreateHPV(volID)
	hostpathVol.SetVolSize(cap)
	hostpathVol.SetVolPathAnchorDir(anchorDir)
	hostpathVol.SetVolPathBindMountDir(bindDir)
	hostpathVol.SetVolAccessType(volAccessType)
	hostpathVol.SetTargetPath(targetPath)
	hostpathVol.SetPodNamespace(podNamespace)
	hostpathVol.SetPodName(podName)
	hostpathVol.SetPodUID(podUID)
	hostpathVol.SetPodSA(podSA)
	hostpathVol.SetRefresh(refresh)
	switch {
	case cmShare != nil:
		hostpathVol.SetSharedDataKind(string(consts.ResourceReferenceTypeConfigMap))
		hostpathVol.SetSharedDataId(cmShare.Name)
	case sShare != nil:
		hostpathVol.SetSharedDataKind(string(consts.ResourceReferenceTypeSecret))
		hostpathVol.SetSharedDataId(sShare.Name)
	}
	hostpathVol.SetReadOnly(readOnly)

	return hostpathVol, nil
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		klog.Warningf("error opening %s during empty check: %s", name, err.Error())
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func deleteIfEmpty(name string) {
	if empty, err := isDirEmpty(name); empty && err == nil {
		klog.V(4).Infof("deleteIfEmpty %s", name)
		err = os.RemoveAll(name)
		if err != nil {
			klog.Warningf("error deleting %s: %s", name, err.Error())
		}
	}
}

func (hp *hostPath) innerDeleteHostpathVolume(top string) {
	// reminder, path is filepath.Join(DataRoot, [anchor-dir | bind-dir], volID, podNamespace, podName, podUID, podSA)
	// delete SA dir
	klog.V(4).Infof("innerDeleteHostpathVolume %s", top)
	err := os.RemoveAll(top)
	if err != nil {
		klog.Warningf("error deleting %s: %s", top, err.Error())
	}
	currentLocation := top
	// we deleteIfEmpty on the remaining 4 levels
	for i := 0; i < 4; i++ {
		parentDir := filepath.Dir(currentLocation)
		deleteIfEmpty(parentDir)
		currentLocation = parentDir
	}
}

// deleteVolume deletes the directory for the hostpath volume.
func (hp *hostPath) deleteHostpathVolume(volID string) error {
	klog.V(4).Infof("deleting hostpath volume: %s", volID)

	if hpv := hp.getHostpathVolume(volID); hpv != nil {
		klog.V(4).Infof("found volume: %s", volID)
		os.RemoveAll(hpv.GetTargetPath())
		if hpv.IsReadOnly() {
			hp.innerDeleteHostpathVolume(hpv.GetVolPathBindMountDir())
		}
		remHPV(volID)
	}
	objcache.UnregisterSecretUpsertCallback(volID)
	objcache.UnregisterSecretDeleteCallback(volID)
	objcache.UnregisterConfigMapUpsertCallback(volID)
	objcache.UnregisterConfigMapDeleteCallback(volID)
	objcache.UnregisterSharedConfigMapDeleteCallback(volID)
	objcache.UnregisterSharedConfigMapUpdateCallback(volID)
	objcache.UnregisterSharedSecretDeleteCallback(volID)
	objcache.UnregsiterSharedSecretsUpdateCallback(volID)
	return nil
}

func (hp *hostPath) loadVolsFromDisk() error {
	klog.V(2).Infof("loadVolsFromDisk")
	defer klog.V(2).Infof("loadVolsFromDisk exit")
	return filepath.Walk(VolumeMapRoot, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if err != nil {
			// continue to next file
			return nil
		}
		if info.IsDir() {
			return nil
		}
		fileName := filepath.Join(VolumeMapRoot, info.Name())
		dataFile, oerr := os.Open(fileName)
		if oerr != nil {
			klog.V(0).Infof("loadVolsFromDisk error opening file %s: %s", fileName, err.Error())
			// continue to next file
			return nil
		}
		dataDecoder := json.NewDecoder(dataFile)
		hpv := &hostPathVolume{}
		err = dataDecoder.Decode(hpv)
		if err != nil {
			klog.V(0).Infof("loadVolsFromDisk error decoding file %s: %s", fileName, err.Error())
			// continue to next file
			return nil
		}
		if hpv == nil {
			klog.V(0).Infof("loadVolsFromDisk nil but no error for file %s", fileName)
			// continue to next file
			return nil
		}
		hpv.Lock = &sync.Mutex{}
		if filepath.Base(fileName) != hpv.GetVolID() {
			klog.Warningf("loadVolsFromDisk file %s had vol id %s - corrupted !!!", hpv.GetVolID())
			return nil
		}
		klog.V(2).Infof("loadVolsFromDisk storing with key %s hpv %#v", hpv.GetVolID(), hpv)
		setHPV(hpv.GetVolID(), hpv)
		hp.registerRangers(hpv)

		return nil
	})
}
