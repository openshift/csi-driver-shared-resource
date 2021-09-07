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
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	storagev1alpha1 "github.com/openshift/api/storage/v1alpha1"

	"github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
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

	// kubeClient optional clientset, when informed the driver will employ it to update the cache
	// based on the Share's backing-resource.
	kubeClient kubernetes.Interface
}

var (
	vendorVersion = "dev"

	hostPathVolumes = sync.Map{}

	fileWriteLock = sync.Mutex{}

	volMapOnDiskPath = filepath.Join(VolumeMapRoot, VolumeMapFile)
)

const (
	// Directory where data for volumes are persisted.
	// This is ephemeral to facilitate our per-pod, tmpfs,
	// no bind mount, approach.
	DataRoot = "/csi-data-dir"

	// Directory where we persist `hostPathVolumes`
	// This is a hostpath volume on the local node
	// to maintain state across restarts of the DaemonSet
	VolumeMapRoot = "/csi-volumes-map"
	VolumeMapFile = "volumemap.json"
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
	createHostpathVolume(volID, targetPath string, readOnly bool, volCtx map[string]string, share *storagev1alpha1.SharedResource, cap int64, volAccessType accessType) (*hostPathVolume, error)
	getHostpathVolume(volID string) *hostPathVolume
	deleteHostpathVolume(volID string) error
	getVolumePath(volID string, volCtx map[string]string) (string, string)
	mapVolumeToPod(hpv *hostPathVolume) error
}

// NewHostPathDriver instantiate the HostPathDriver with the driver details.  Optionally, a
// Kubernetes Clientset can be informed to update (warm up) the object cache before creating the
// volume (and it's data) for mounting on the incoming pod.
func NewHostPathDriver(root, volMapRoot, driverName, nodeID, endpoint string, maxVolumesPerNode int64, version string, kubeClient kubernetes.Interface) (*hostPath, error) {
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

	klog.Infof("Driver: %v ", driverName)
	klog.Infof("Version: %s", vendorVersion)

	if kubeClient != nil {
		klog.Info("HostPathDriver will directly read Kubernetes resources!")
	}

	hp := &hostPath{
		name:              driverName,
		version:           vendorVersion,
		nodeID:            nodeID,
		endpoint:          endpoint,
		maxVolumesPerNode: maxVolumesPerNode,
		root:              root,
		kubeClient:        kubeClient,
	}

	volMapOnDiskPath = filepath.Join(volMapRoot, VolumeMapFile)
	if err := hp.loadVolMapFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load volume map on disk: %v", err)
	}

	return hp, nil
}

func (hp *hostPath) Run() {
	// Create GRPC servers
	hp.ids = NewIdentityServer(hp.name, hp.version)

	// the node-server will be on always-read-only mode when the object-cache is being populated
	// directly, which happens when an instantiated kubeClient is informed to this component
	alwaysReadOnly := hp.kubeClient != nil
	hp.ns = NewNodeServer(hp, alwaysReadOnly)

	s := NewNonBlockingGRPCServer()
	s.Start(hp.endpoint, hp.ids, hp.ns)
	s.Wait()
}

// getVolumePath returns the canonical paths for hostpath volume
func (hp *hostPath) getVolumePath(volID string, volCtx map[string]string) (string, string) {
	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	return filepath.Join(hp.root, anchorDir, volID, podNamespace, podName, podUID, podSA), filepath.Join(hp.root, bindDir, volID, podNamespace, podName, podUID, podSA)
}

func commonUpsertRanger(obj runtime.Object, podPath, filter string, key, value interface{}) error {
	if key != filter {
		return nil
	}
	payload, _ := value.(Payload)
	klog.V(4).Infof("common upsert ranger key %s", key)
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
	if err := commonOSRemove(podPath); err != nil {
		return err
	}
	if err := os.MkdirAll(podPath, os.ModePerm); err != nil {
		return err
	}
	if payload.ByteData != nil {
		for dataKey, dataValue := range payload.ByteData {
			podFilePath := filepath.Join(podPath, dataKey)
			klog.V(4).Infof("create/update file %s", podFilePath)
			if err := ioutil.WriteFile(podFilePath, dataValue, 0644); err != nil {
				return err
			}

		}
	}
	if payload.StringData != nil {
		for dataKey, dataValue := range payload.StringData {
			podFilePath := filepath.Join(podPath, dataKey)
			klog.V(4).Infof("create/update file %s", podFilePath)
			content := []byte(dataValue)
			if err := ioutil.WriteFile(podFilePath, content, 0644); err != nil {
				return err
			}
		}
	}
	klog.V(4).Infof("common upsert ranger returning key %s", key)
	return nil
}

func commonOSRemove(dir string) error {
	klog.V(4).Infof("attempting to delete %q", dir)
	defer klog.V(4).Infof("completed delete attempt for %q", dir)
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
		klog.V(4).Infof("commonOSRemove %s", fileName)
		return os.RemoveAll(fileName)
	})

}

func commonDeleteRanger(podPath, filter string, key interface{}) bool {
	if key != filter {
		return true
	}
	klog.V(4).Infof("common delete ranger key %s", key)
	commonOSRemove(podPath)
	klog.V(4).Infof("common delete ranger returning key %s", key)
	return true
}

func shareDeleteRanger(hp *hostPath, key interface{}) bool {
	shareId := key.(string)
	targetPath := ""
	volID := ""
	klog.V(4).Infof("share delete ranger share id %s", shareId)
	ranger := func(key, value interface{}) bool {
		hpv, _ := value.(*hostPathVolume)
		if hpv.GetSharedDataId() == shareId {
			klog.V(4).Infof("shareDeleteRanger shareid %s", shareId)
			if hpv.IsReadOnly() {
				targetPath = hpv.GetVolPathBindMountDir()

			} else {
				targetPath = hpv.GetTargetPath()
			}
			volID = hpv.GetVolID()
			// deleting the share effectively deletes permission to the
			// data so we set the allowed bit to false; this will have bearing
			// if the share is added again at a later date and the associated
			// pod in question is still up
			hpv.SetAllowed(false)
			return false
		}
		return true
	}
	hostPathVolumes.Range(ranger)
	if len(volID) > 0 && len(targetPath) > 0 {
		err := commonOSRemove(targetPath)
		if err != nil {
			klog.Warningf("share %s vol %s target path %s delete error %s",
				shareId, volID, targetPath, err.Error())
		}
		// we just delete the associated data from the previously provisioned volume;
		// we don't delete the volume in case the share is added back
		storeVolMapToDisk()
	}
	klog.V(4).Infof("share delete ranger returning share id %s", shareId)
	return true
}

func shareUpdateRanger(key, value interface{}) bool {
	shareId := key.(string)
	share := value.(*storagev1alpha1.SharedResource)
	klog.V(4).Infof("share update ranger id %s share name %s type %s version %s", shareId, share.Name, share.Spec.Resource.Type, share.ResourceVersion)
	oldTargetPath := ""
	volID := ""
	change := false
	newVersion := false
	lostPermissions := false
	gainedPermissions := false
	var hpv *hostPathVolume
	ranger := func(key, value interface{}) bool {
		hpv, _ = value.(*hostPathVolume)
		klog.V(4).Infof("share update ranger id %s share name %s type %s hpv ranger", shareId, share.Name, share.Spec.Resource.Type)
		if hpv.GetSharedDataId() == shareId {
			klog.V(4).Infof("share update ranger id %s share name %s type %s hpv ranger found volume %s", shareId, share.Name, share.Spec.Resource.Type, hpv.GetVolID())
			a, err := client.ExecuteSAR(shareId, hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetPodSA())
			allowed := a && err == nil

			if allowed && !hpv.IsAllowed() {
				klog.V(0).Infof("pod %s:%s regained permissions for share %s",
					hpv.GetPodNamespace(), hpv.GetPodName(), shareId)
				gainedPermissions = true
				hpv.SetAllowed(true)
			}
			if !allowed && hpv.IsAllowed() {
				klog.V(0).Infof("pod %s:%s no longer has permission for share %s",
					hpv.GetPodNamespace(), hpv.GetPodName(), shareId)
				lostPermissions = true
				hpv.SetAllowed(false)
			}

			newVersion = hpv.CheckBeforeSetSharedDataVersion(share.ResourceVersion)
			if !newVersion {
				klog.V(0).Infof("share %s at version %s for pod %s:%s will be ignored because version %s has been received",
					shareId, share.ResourceVersion, hpv.GetPodNamespace(), hpv.GetPodName(), hpv.GetSharedDataVersion())
			}

			switch {
			case share.Spec.Resource.Type != hpv.GetSharedDataType():
				change = true
			case cache.GetKeyFrom(share.Spec.Resource) != hpv.GetSharedDataKey():
				change = true
			}
			if !change && !lostPermissions && !gainedPermissions {
				return false
			}
			if hpv.IsReadOnly() {
				oldTargetPath = hpv.GetVolPathBindMountDir()
			} else {
				oldTargetPath = hpv.GetTargetPath()
			}
			volID = hpv.GetVolID()
			return false
		}
		return true
	}
	hostPathVolumes.Range(ranger)

	klog.V(4).Infof("share update ranger id %s share name %s type %s, ranged over hpv's: lostPermissions %v change %v gainedPermission %v",
		shareId, share.Name, share.Spec.Resource.Type, lostPermissions, change, gainedPermissions)

	if lostPermissions {
		err := commonOSRemove(oldTargetPath)
		if err != nil {
			klog.Warningf("share %s vol %s target path %s delete error %s",
				shareId, volID, oldTargetPath, err.Error())
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
			klog.V(4).Infof("shareUpdateRanger kubelet dir %s empty %v err %s", tp, empty, errStr)
			if !empty {
				err = commonOSRemove(tp)
				if err != nil {
					errStr = err.Error()
				}
				klog.V(4).Infof("shareUpdateRanger kubelet dir %s commonOsRemove err %s", tp, errStr)

			}
		}
		cache.UnregisterSecretUpsertCallback(volID)
		cache.UnregisterSecretDeleteCallback(volID)
		cache.UnregisterConfigMapDeleteCallback(volID)
		cache.UnregisterConfigMapUpsertCallback(volID)
		storeVolMapToDisk()
		return true
	}

	if change && newVersion {
		err := commonOSRemove(oldTargetPath)
		if err != nil {
			klog.Warningf("share %s vol %s target path %s delete error %s",
				shareId, volID, oldTargetPath, err.Error())
		}
		cache.UnregisterSecretUpsertCallback(volID)
		cache.UnregisterSecretDeleteCallback(volID)
		cache.UnregisterConfigMapDeleteCallback(volID)
		cache.UnregisterConfigMapUpsertCallback(volID)

		hpv.SetSharedDataType(share.Spec.Resource.Type)
		hpv.SetSharedDataKey(cache.GetKeyFrom(share.Spec.Resource))
		hpv.SetSharedDataId(share.Name)

		mapResourceToPod(hpv)
	}

	if gainedPermissions {
		mapResourceToPod(hpv)
	}

	if (change && newVersion) || gainedPermissions {
		storeVolMapToDisk()
	}

	klog.V(4).Infof("share update ranger id %s share name %s type %s version %s returning", shareId, share.Name, share.Spec.Resource.Type, share.ResourceVersion)
	return true
}

func mapResourceToPod(hpv *hostPathVolume) error {
	klog.V(4).Infof("mapResourceToPod")
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
	err := os.MkdirAll(hpv.GetVolPathAnchorDir(), 0777)
	if err != nil {
		return err
	}
	switch hpv.GetSharedDataType() {
	case storagev1alpha1.ResourceReferenceTypeConfigMap:
		klog.V(4).Infof("mapResourceToPod postlock %s configmap", hpv.GetVolID())
		podConfigMapsPath := hpv.GetTargetPath()
		if readOnly {
			podConfigMapsPath = hpv.GetVolPathBindMountDir()
		}
		upsertRangerCM := func(key, value interface{}) bool {
			cm, _ := value.(*corev1.ConfigMap)
			payload := Payload{
				StringData: cm.Data,
				ByteData:   cm.BinaryData,
			}
			err := commonUpsertRanger(cm, podConfigMapsPath, hpv.GetSharedDataKey(), key, payload)
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
		cm := cache.GetConfigMap(hpv.GetSharedDataKey())
		if cm != nil {
			payload := Payload{
				StringData: cm.Data,
				ByteData:   cm.BinaryData,
			}

			upsertError := commonUpsertRanger(cm, podConfigMapsPath, hpv.GetSharedDataKey(), hpv.GetSharedDataKey(), payload)
			if upsertError != nil {
				ProcessFileSystemError(cm, upsertError)
				return upsertError
			}
		}
		cache.RegisterConfigMapUpsertCallback(hpv.GetVolID(), upsertRangerCM)
		deleteRangerCM := func(key, value interface{}) bool {
			return commonDeleteRanger(podConfigMapsPath, hpv.GetSharedDataKey(), key)
		}
		cache.RegisterConfigMapDeleteCallback(hpv.GetVolID(), deleteRangerCM)
	case storagev1alpha1.ResourceReferenceTypeSecret:
		klog.V(4).Infof("mapResourceToPod postlock %s secret", hpv.GetVolID())
		podSecretsPath := hpv.GetTargetPath()
		if readOnly {
			podSecretsPath = hpv.GetVolPathBindMountDir()
		}
		upsertRangerSec := func(key, value interface{}) bool {
			s, _ := value.(*corev1.Secret)
			payload := Payload{
				ByteData: s.Data,
			}
			err := commonUpsertRanger(s, podSecretsPath, hpv.GetSharedDataKey(), key, payload)
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
		s := cache.GetSecret(hpv.GetSharedDataKey())
		if s != nil {
			payload := Payload{
				ByteData: s.Data,
			}

			upsertError := commonUpsertRanger(s, podSecretsPath, hpv.GetSharedDataKey(), hpv.GetSharedDataKey(), payload)
			if upsertError != nil {
				ProcessFileSystemError(s, upsertError)
				return upsertError
			}
		}
		cache.RegisterSecretUpsertCallback(hpv.GetVolID(), upsertRangerSec)
		deleteRangerSec := func(key, value interface{}) bool {
			return commonDeleteRanger(podSecretsPath, hpv.GetSharedDataKey(), key)
		}
		cache.RegisterSecretDeleteCallback(hpv.GetVolID(), deleteRangerSec)
	default:
		return fmt.Errorf("invalid share backing resource kind %s", hpv.GetSharedDataType())
	}
	return nil
}

// updateObjCache fetches the resources and populates the object-cache just before mounting.
func (hp *hostPath) updateObjCache(hpv *hostPathVolume) error {
	kind := hpv.GetSharedDataType()
	key := hpv.GetSharedDataKey()
	klog.V(4).Infof("populating object-cache with '%s' (key='%s') before mounting", kind, key)
	switch kind {
	case "ConfigMap":
		return cache.SetConfigMap(hp.kubeClient, key)
	case "Secret":
		return cache.SetSecret(hp.kubeClient, key)
	default:
		return fmt.Errorf("invalid share backing resource kind %s", kind)
	}
}

func (hp *hostPath) mapVolumeToPod(hpv *hostPathVolume) error {
	klog.V(4).Infof("mapVolumeToPod calling mapBackingResourceToPod")

	// given the kubeclient is instantiated, it will use it to fetch the resources just before
	// mounting the volume on the pod, otherwise, it's exected the object-cache already contains the
	// resource in question
	if hp.kubeClient != nil {
		if err := hp.updateObjCache(hpv); err != nil {
			return err
		}
	}

	err := mapResourceToPod(hpv)
	if err != nil {
		return err
	}
	deleteRangerShare := func(key, value interface{}) bool {
		return shareDeleteRanger(hp, key)
	}
	cache.RegisterShareDeleteCallback(hpv.GetVolID(), deleteRangerShare)
	updateRangerShare := func(key, value interface{}) bool {
		return shareUpdateRanger(key, value)
	}
	cache.RegisterShareUpdateCallback(hpv.GetVolID(), updateRangerShare)

	return nil
}

// createVolume create the directory for the hostpath volume.
// It returns the volume path or err if one occurs.
func (hp *hostPath) createHostpathVolume(volID, targetPath string, readOnly bool, volCtx map[string]string, share *storagev1alpha1.SharedResource, cap int64, volAccessType accessType) (*hostPathVolume, error) {
	fileWriteLock.Lock()
	defer fileWriteLock.Unlock()
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
	hostpathVol.SetSharedDataType(share.Spec.Resource.Type)
	hostpathVol.SetSharedDataKey(cache.GetKeyFrom(share.Spec.Resource))
	hostpathVol.SetSharedDataId(share.Name)
	hostpathVol.SetSharedDataVersion(share.ResourceVersion)
	hostpathVol.SetAllowed(true)
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
		err = os.RemoveAll(name)
		if err != nil {
			klog.Warningf("error deleting %s: %s", name, err.Error())
		}
	}
}

func (hp *hostPath) innerDeleteHostpathVolume(top string) {
	// reminder, path is filepath.Join(DataRoot, [anchor-dir | bind-dir], volID, podNamespace, podName, podUID, podSA)
	// delete SA dir
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
		klog.V(4).Infof("deleting hostpath volume: %s", volID)
		os.RemoveAll(hpv.GetTargetPath())
		hp.innerDeleteHostpathVolume(hpv.GetVolPathBindMountDir())
		hp.innerDeleteHostpathVolume(hpv.GetVolPathAnchorDir())
		remHPV(volID)
		storeVolMapToDisk()
	}
	cache.UnregisterSecretUpsertCallback(volID)
	cache.UnregisterSecretDeleteCallback(volID)
	cache.UnregisterConfigMapDeleteCallback(volID)
	cache.UnregisterConfigMapUpsertCallback(volID)
	cache.UnregisterShareDeleteCallback(volID)
	cache.UnregisterShareUpdateCallback(volID)
	return nil
}

func storeVolMapToDisk() error {
	klog.V(4).Info("storeVolMapToDisk prelock")
	fileWriteLock.Lock()
	defer fileWriteLock.Unlock()
	klog.V(4).Info("storeVolMapToDisk postlock")
	dataFile, err := os.Create(volMapOnDiskPath)
	if err != nil {
		klog.Warningf("error creating map file: %s", err.Error())
		return err
	}
	defer dataFile.Close()
	dataEncoder := json.NewEncoder(dataFile)
	mapCopy := map[string]hostPathVolume{}
	ranger := func(key, value interface{}) bool {
		v, _ := value.(*hostPathVolume)
		k, _ := key.(string)
		mapCopy[k] = *v
		return true
	}
	hostPathVolumes.Range(ranger)
	return dataEncoder.Encode(mapCopy)
}

func (hp *hostPath) loadVolMapFromDisk() error {
	klog.V(2).Infof("loadVolMapFromDisk")
	dataFile, err := os.Open(volMapOnDiskPath)
	if os.IsNotExist(err) {
		return storeVolMapToDisk()
	}
	if err != nil {
		klog.Warningf("error opening map file: %s", err.Error())
		return err
	}
	defer dataFile.Close()
	mapCopy := map[string]hostPathVolume{}
	dataDecoder := json.NewDecoder(dataFile)
	err = dataDecoder.Decode(&mapCopy)
	if err != nil {
		klog.Warningf("error decoding map file: %s", err.Error())
		return err
	}
	hostPathVolumes = sync.Map{}
	for k, v := range mapCopy {
		klog.V(4).Infof("loadVolMapFromDisk looking at volume %s hpv %#v", k, v)
		// call this first to establish locking
		setHPV(k, &v)
		pod, err := client.GetPod(v.GetPodNamespace(), v.GetPodName())
		if err != nil {
			klog.V(2).Infof("loadVolMapFromDisk could not find pod %s:%s so dropping: %s",
				v.GetPodNamespace(), v.GetPodName(), err.Error())
			remHPV(k)
			continue
		}
		if string(pod.UID) != v.GetPodUID() {
			klog.V(2).Infof("loadVolMapFromDisk found pod %s:%s but UIDs do no match so dropping: %s vs %s",
				v.GetPodNamespace(), v.GetPodName(), string(pod.UID), v.GetPodUID())
			remHPV(k)
			continue
		}
		err = hp.mapVolumeToPod(&v)
		if err != nil {
			klog.Warningf("loadVolMapFromDisk error mapping volume %s to shares: %s", k, err.Error())
		}
	}
	return nil
}
