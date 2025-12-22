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

package csidriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	atomic "k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/utils/mount"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"

	objcache "github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/config"
	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
)

type driver struct {
	name              string
	nodeID            string
	version           string
	endpoint          string
	ephemeral         bool
	maxVolumesPerNode int64

	ids *identityServer
	ns  *nodeServer

	root       string
	volMapRoot string

	mounter mount.Interface
}

var (
	vendorVersion = "dev"

	volumes = sync.Map{}
)

const (
	// Directory where data for volumes are persisted.
	// This is ephemeral to facilitate our per-pod, tmpfs,
	// no bind mount, approach.
	DataRoot = "/run/csi-data-dir"

	// Directory where we persist `volumes`
	// This is a csidriver volume on the local node
	// to maintain state across restarts of the DaemonSet
	VolumeMapRoot = "/csi-volumes-map"
)

func (d *driver) getVolume(name string) *driverVolume {
	obj, loaded := volumes.Load(name)
	if loaded {
		dv, _ := obj.(*driverVolume)
		return dv
	}
	return nil
}

func setDPV(name string, dpv *driverVolume) {
	if dpv.Lock == nil {
		dpv.Lock = &sync.Mutex{}
	}
	volumes.Store(name, dpv)
}

func remV(name string) {
	volumes.Delete(name)
}

type CSIDriver interface {
	createVolume(volID, targetPath string, refresh bool, volCtx map[string]string, cmShare *sharev1alpha1.SharedConfigMap, sShare *sharev1alpha1.SharedSecret, cap int64, volAccessType accessType) (*driverVolume, error)
	getVolume(volID string) *driverVolume
	deleteVolume(volID string) error
	getVolumePath(volID string, volCtx map[string]string) (string, string)
	mapVolumeToPod(dv *driverVolume) error
	Run(rn *config.ReservedNames)
	GetRoot() string
	GetVolMapRoot() string
	Prune(kubeClient kubernetes.Interface)
}

// NewCSIDriver instantiate the CSIDriver with the driver details.  Optionally, a
// Kubernetes Clientset can be informed to update (warm up) the object cache before creating the
// volume (and it's data) for mounting on the incoming pod.
func NewCSIDriver(root, volMapRoot, driverName, nodeID, endpoint string, maxVolumesPerNode int64, version string, mounter mount.Interface) (CSIDriver, error) {
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
		klog.Info("RefreshResources is disabled and CSIDriver will directly read Kubernetes corev1 resources!")
	}

	d := &driver{
		name:              driverName,
		version:           vendorVersion,
		nodeID:            nodeID,
		endpoint:          endpoint,
		maxVolumesPerNode: maxVolumesPerNode,
		root:              root,
		volMapRoot:        volMapRoot,
		mounter:           mounter,
	}

	if err := d.loadVolsFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load volume map on disk: %v", err)
	}

	return d, nil
}

func (d *driver) GetRoot() string {
	return d.root
}

func (d *driver) GetVolMapRoot() string {
	return d.volMapRoot
}

func (d *driver) Run(rn *config.ReservedNames) {
	// Create GRPC servers
	d.ids = NewIdentityServer(d.name, d.version)

	// the node-server will be on always-read-only mode when the object-cache is being populated
	// directly
	d.ns = NewNodeServer(d, rn)

	s := NewNonBlockingGRPCServer()
	s.Start(d.endpoint, d.ids, d.ns)
	s.Wait()
}

// getVolumePath returns the canonical paths for csidriver volume
func (d *driver) getVolumePath(volID string, volCtx map[string]string) (string, string) {
	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	mountIDString := strings.Join([]string{podNamespace, podName, volID}, "-")
	return mountIDString, filepath.Join(d.root, bindDir, volID, podNamespace, podName, podUID, podSA)
}

func commonRangerProceedFilter(dv *driverVolume, key interface{}) bool {
	if dv == nil {
		return false
	}
	compareKey := ""
	// see if the shared item pertains to this volume
	switch dv.GetSharedDataKind() {
	case consts.ResourceReferenceTypeSecret:
		sharedSecret := client.GetSharedSecret(dv.GetSharedDataId())
		if sharedSecret == nil {
			klog.V(6).Infof("commonRangerProceedFilter could not retrieve share %s for %s:%s:%s", dv.GetSharedDataId(), dv.GetPodNamespace(), dv.GetPodName(), dv.GetVolID())
			return false
		}
		compareKey = objcache.BuildKey(sharedSecret.Spec.SecretRef.Namespace, sharedSecret.Spec.SecretRef.Name)
	case consts.ResourceReferenceTypeConfigMap:
		sharedConfigMap := client.GetSharedConfigMap(dv.GetSharedDataId())
		if sharedConfigMap == nil {
			klog.V(6).Infof("commonRangerProceedFilter could not retrieve share %s for %s:%s:%s", dv.GetSharedDataId(), dv.GetPodNamespace(), dv.GetPodName(), dv.GetVolID())
			return false
		}
		compareKey = objcache.BuildKey(sharedConfigMap.Spec.ConfigMapRef.Namespace, sharedConfigMap.Spec.ConfigMapRef.Name)
	default:
		klog.Warningf("commonRangerProceedFilter unknown share type for %s:%s:%s", dv.GetPodNamespace(), dv.GetPodName(), dv.GetVolID())
		return false
	}
	keyStr := key.(string)
	if keyStr != compareKey {
		klog.V(4).Infof("commonRangerProceedFilter skipping %s as it does not match %s for %s:%s:%s", keyStr, compareKey, dv.GetPodNamespace(), dv.GetPodName(), dv.GetVolID())
		return false
	}
	return true
}

func commonUpsertRanger(dv *driverVolume, key, value interface{}) error {
	proceed := commonRangerProceedFilter(dv, key)
	if !proceed {
		return nil
	}

	payload, _ := value.(Payload)
	klog.V(4).Infof("commonUpsertRanger key %s dv %#v", key, dv)
	podPath := dv.GetTargetPath()
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

	// NOTE: atomic_writer handles any pruning of secret/configmap keys that were present before, but are no longer
	// present
	if err := os.MkdirAll(podPath, os.ModePerm); err != nil {
		return err
	}
	podFile := map[string]atomic.FileProjection{}
	aw, err := atomic.NewAtomicWriter(podPath, "shared-resource-csi-driver")
	if err != nil {
		return err
	}
	if payload.ByteData != nil {
		for dataKey, dataValue := range payload.ByteData {
			podFilePath := filepath.Join(podPath, dataKey)
			klog.V(4).Infof("commonUpsertRanger create/update file %s key %s volid %s share id %s pod name %s", podFilePath, key, dv.GetVolID(), dv.GetSharedDataId(), dv.GetPodName())
			podFile[dataKey] = atomic.FileProjection{
				Data: dataValue,
				Mode: 0644,
			}
		}
	}
	if payload.StringData != nil {
		for dataKey, dataValue := range payload.StringData {
			podFilePath := filepath.Join(podPath, dataKey)
			klog.V(4).Infof("commonUpsertRanger create/update file %s key %s volid %s share id %s pod name %s", podFilePath, key, dv.GetVolID(), dv.GetSharedDataId(), dv.GetPodName())
			content := []byte(dataValue)
			podFile[dataKey] = atomic.FileProjection{
				Data: content,
				Mode: 0644,
			}
		}
	}
	if len(podFile) > 0 {
		if err = aw.Write(podFile, nil); err != nil {
			return err
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
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	dirEntries, err := dirFile.ReadDir(-1)
	if err != nil {
		return err
	}
	// we have to unlink the key from our configmap/secret first with the atomic writer symlinks
	for _, dirEntry := range dirEntries {
		if dirEntry.Type()&os.ModeSymlink != 0 && !strings.HasPrefix(dirEntry.Name(), "..") {
			// unlink our secret/configmap key
			if err = syscall.Unlink(filepath.Join(dir, dirEntry.Name())); err != nil {
				klog.Errorf("commonOSRemove encountered an error: %s", err.Error())
				return err
			}
		}
	}
	// we then unlink the symlink from atomic writer that start with ".."
	for _, dirEntry := range dirEntries {
		if dirEntry.Type()&os.ModeSymlink != 0 && strings.HasPrefix(dirEntry.Name(), "..") {
			if err = syscall.Unlink(filepath.Join(dir, dirEntry.Name())); err != nil {
				klog.Errorf("commonOSRemove encountered an error: %s", err.Error())
				return err
			}

		}
		// then we delete any non symlinks
		if dirEntry.Type()&os.ModeSymlink == 0 {
			fileName := filepath.Join(dir, dirEntry.Name())
			klog.V(4).Infof("commonOSRemove going to delete file %s", fileName)
			if err = os.RemoveAll(fileName); err != nil {
				klog.Errorf("commonOSRemove encountered an error: %s", err.Error())
				return err
			}
		}
	}
	return nil
}

func commonDeleteRanger(dv *driverVolume, key interface{}) bool {
	proceed := commonRangerProceedFilter(dv, key)
	if !proceed {
		// even though we are aborting, return true to continue to next entry in ranger list
		return true
	}
	klog.V(4).Infof("common delete ranger key %s", key)
	commonOSRemove(dv.GetTargetPath(), fmt.Sprintf("commonDeleteRanger %s", key))
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
	var dv *driverVolume
	klog.V(4).Infof("innerShareDeleteRanger key %q\n incoming share id %s",
		key,
		r.shareId)
	dvObj, ok := volumes.Load(key)
	if !ok {
		klog.V(0).Infof("innerShareDeleteRanger how the hell can we not load key %s from the range list", key)
		// continue to the next entry, skip this one
		return true
	} else {
		dv, _ = dvObj.(*driverVolume)
	}
	if dv.GetVolID() == volID && dv.GetSharedDataId() == r.shareId {
		klog.V(4).Infof("innerShareDeleteRanger shareid %s kind %s", r.shareId, dv.GetSharedDataKind())
		targetPath = dv.GetTargetPath()
		volID = dv.GetVolID()
		if len(volID) > 0 && len(targetPath) > 0 {
			err := commonOSRemove(targetPath, fmt.Sprintf("innerShareDeleteRanger shareID id %s", r.shareId))
			if err != nil {
				klog.Warningf("innerShareDeleteRanger %s vol %s target path %s delete error %s",
					r.shareId, volID, targetPath, err.Error())
			}
			// we just delete the associated data from the previously provisioned volume;
			// we don't delete the volume in case the share is added back
		}
		return true
	}
	return true
}

func shareDeleteRanger(key interface{}) bool {
	shareId := key.(string)
	klog.V(4).Infof("shareDeleteRanger shareID id %s", shareId)
	ranger := &innerShareDeleteRanger{
		shareId: shareId,
	}

	volumes.Range(ranger.Range)
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
	var dv *driverVolume
	klog.V(4).Infof("innerShareUpdateRanger key %q\n incoming share id %s",
		key,
		r.shareId)
	dvObj, ok := volumes.Load(key)
	if !ok {
		klog.V(0).Infof("innerShareUpdateRanger how the hell can we not load key %s from the range list", key)
		// continue to the next entry, skip this one
		return true
	} else {
		dv, _ = dvObj.(*driverVolume)
	}
	if dv.GetVolID() == volID && dv.GetSharedDataId() == r.shareId {
		klog.V(4).Infof("innerShareUpdateRanger MATCH inner ranger key %q\n dv vol id %s\n incoming share id %s\n dv share id %s", key, dv.GetVolID(), r.shareId, dv.GetSharedDataId())
		a, err := client.ExecuteSAR(r.shareId, dv.GetPodNamespace(), dv.GetPodName(), dv.GetPodSA(), dv.GetSharedDataKind())
		allowed := a && err == nil

		if allowed {
			klog.V(0).Infof("innerShareUpdateRanger pod %s:%s has permissions for secretShare %s",
				dv.GetPodNamespace(), dv.GetPodName(), r.shareId)
		} else {
			klog.V(0).Infof("innerShareUpdateRanger pod %s:%s does not have permission for secretShare %s",
				dv.GetPodNamespace(), dv.GetPodName(), r.shareId)
		}

		switch {
		case r.secret:
			sharedSecret := client.GetSharedSecret(r.shareId)
			if sharedSecret == nil {
				klog.Warningf("innerShareUpdateRanger unexpected not found on sharedSecret lister refresh: %s", r.shareId)
				return true
			}
			r.sharedItemKey = objcache.BuildKey(sharedSecret.Spec.SecretRef.Namespace, sharedSecret.Spec.SecretRef.Name)
			secretObj, err := client.GetSecret(sharedSecret.Spec.SecretRef.Namespace, sharedSecret.Spec.SecretRef.Name)
			if err != nil || secretObj == nil {
				klog.Warningf("innerShareUpdateRanger share %s could not retrieve shared item %s, error: %v", r.shareId, r.sharedItemKey, err)
				return true
			}
			r.sharedItem = Payload{
				ByteData:   secretObj.Data,
				StringData: secretObj.StringData,
			}
		case r.configmap:
			sharedConfigMap := client.GetSharedConfigMap(r.shareId)
			if sharedConfigMap == nil {
				klog.Warningf("innerShareUpdateRanger unexpected not found on sharedConfigMap lister refresh: %s", r.shareId)
				return true
			}
			r.sharedItemKey = objcache.BuildKey(sharedConfigMap.Spec.ConfigMapRef.Namespace, sharedConfigMap.Spec.ConfigMapRef.Name)
			cmObj, err := client.GetConfigMap(sharedConfigMap.Spec.ConfigMapRef.Namespace, sharedConfigMap.Spec.ConfigMapRef.Name)
			if err != nil || cmObj == nil {
				klog.Warningf("innerShareUpdateRanger share %s could not retrieve shared item %s, error: %v", r.shareId, r.sharedItemKey, err)
				return true
			}
			r.sharedItem = Payload{
				StringData: cmObj.Data,
				ByteData:   cmObj.BinaryData,
			}
		}

		r.oldTargetPath = dv.GetTargetPath()
		r.volID = dv.GetVolID()

		if !allowed {
			err := commonOSRemove(r.oldTargetPath, "lostPermissions")
			if err != nil {
				klog.Warningf("innerShareUpdateRanger %s target path %s delete error %s",
					key, r.oldTargetPath, err.Error())
			}
			objcache.UnregisterSecretUpsertCallback(r.volID)
			objcache.UnregisterSecretDeleteCallback(r.volID)
			objcache.UnregisterConfigMapDeleteCallback(r.volID)
			objcache.UnregisterConfigMapUpsertCallback(r.volID)
			return true // Continue the loop for other volumes
		}

		commonUpsertRanger(dv, r.sharedItemKey, r.sharedItem)

	}
	klog.V(4).Infof("innerShareUpdateRanger NO MATCH inner ranger key %q\n dv vol id %s\n incoming share id %s\n dv share id %s", key, dv.GetVolID(), r.shareId, dv.GetSharedDataId())
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
	volumes.Range(rangerObj.Range)

	klog.V(4).Infof("shareUpdateRanger key %s value %#v inner ranger %#v inner ranger", key, value, rangerObj)
	return true
}

func mapBackingResourceToPod(dv *driverVolume) error {
	klog.V(4).Infof("mapBackingResourceToPod")
	switch dv.GetSharedDataKind() {
	case consts.ResourceReferenceTypeConfigMap:
		klog.V(4).Infof("mapBackingResourceToPod postlock %s configmap", dv.GetVolID())
		upsertRangerCM := func(key, value interface{}) bool {
			cm, _ := value.(*corev1.ConfigMap)
			payload := Payload{
				StringData: cm.Data,
				ByteData:   cm.BinaryData,
			}
			err := commonUpsertRanger(dv, key, payload)
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
		sharedConfigMap := client.GetSharedConfigMap(dv.GetSharedDataId())
		if sharedConfigMap == nil {
			klog.V(4).Infof("mapBackingResourceToPod for pod volume %s:%s:%s share %s no longer exists", dv.GetPodNamespace(), dv.GetPodName(), dv.GetVolID(), dv.GetSharedDataId())
			return nil
		}
		cmNamespace := sharedConfigMap.Spec.ConfigMapRef.Namespace
		cmName := sharedConfigMap.Spec.ConfigMapRef.Name
		comboKey := objcache.BuildKey(cmNamespace, cmName)
		cm, err := client.GetConfigMap(cmNamespace, cmName)
		if err != nil {
			if kerrors.IsForbidden(err) {
				return status.Errorf(codes.PermissionDenied, "CSI driver is forbidden to access configmap %s/%s: %v", cmNamespace, cmName, err)
			}
			// Translate any other error to gRPC Internal
			return status.Errorf(codes.Internal, "CSI driver failed to get configmap %s/%s: %v", cmNamespace, cmName, err)
		}
		if cm != nil {
			payload := Payload{
				StringData: cm.Data,
				ByteData:   cm.BinaryData,
			}

			upsertError := commonUpsertRanger(dv, comboKey, payload)
			if upsertError != nil {
				ProcessFileSystemError(cm, upsertError)
				return upsertError
			}
		}
		if dv.IsRefresh() {
			objcache.RegisterConfigMapUpsertCallback(dv.GetVolID(), comboKey, upsertRangerCM)
		}
		deleteRangerCM := func(key, value interface{}) bool {
			return commonDeleteRanger(dv, key)
		}
		//we should register delete callbacks regardless of any per volume refresh setting to account for removed permissions
		objcache.RegisterConfigMapDeleteCallback(dv.GetVolID(), deleteRangerCM)
	case consts.ResourceReferenceTypeSecret:
		klog.V(4).Infof("mapBackingResourceToPod postlock %s secret", dv.GetVolID())
		upsertRangerSec := func(key, value interface{}) bool {
			s, _ := value.(*corev1.Secret)
			payload := Payload{
				ByteData: s.Data,
			}
			err := commonUpsertRanger(dv, key, payload)
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
		sharedSecret := client.GetSharedSecret(dv.GetSharedDataId())
		sNamespace := sharedSecret.Spec.SecretRef.Namespace
		sName := sharedSecret.Spec.SecretRef.Name
		comboKey := objcache.BuildKey(sNamespace, sName)
		s, err := client.GetSecret(sNamespace, sName)
		if err != nil {
			if kerrors.IsForbidden(err) {
				// Translate Forbidden to gRPC PermissionDenied
				return status.Errorf(codes.PermissionDenied, "CSI driver is forbidden to access secret %s/%s: %v", sNamespace, sName, err)
			}
			// Translate any other error to gRPC Internal
			return status.Errorf(codes.Internal, "CSI driver failed to get secret %s/%s: %v", sNamespace, sName, err)
		}
		if s != nil {
			payload := Payload{
				ByteData: s.Data,
			}

			upsertError := commonUpsertRanger(dv, comboKey, payload)
			if upsertError != nil {
				ProcessFileSystemError(s, upsertError)
				return upsertError
			}
		}
		if dv.IsRefresh() {
			objcache.RegisterSecretUpsertCallback(dv.GetVolID(), comboKey, upsertRangerSec)
		}
		deleteRangerSec := func(key, value interface{}) bool {
			return commonDeleteRanger(dv, key)
		}
		//we should register delete callbacks regardless of any per volume refresh setting to account for removed permissions
		objcache.RegisterSecretDeleteCallback(dv.GetVolID(), deleteRangerSec)
	default:
		return fmt.Errorf("invalid share backing resource kind %s", dv.GetSharedDataKind())
	}
	return nil
}

func (d *driver) mapVolumeToPod(dv *driverVolume) error {
	klog.V(4).Infof("mapVolumeToPod calling mapBackingResourceToPod")

	err := mapBackingResourceToPod(dv)
	if err != nil {
		return err
	}
	d.registerRangers(dv)

	return nil
}

func (d *driver) registerRangers(dv *driverVolume) {
	deleteRangerShare := func(key, value interface{}) bool {
		return shareDeleteRanger(key)
	}
	updateRangerShare := func(key, value interface{}) bool {
		return shareUpdateRanger(key, value)
	}
	switch dv.GetSharedDataKind() {
	case consts.ResourceReferenceTypeSecret:
		objcache.RegisterSharedSecretUpdateCallback(dv.GetVolID(), dv.GetSharedDataId(), updateRangerShare)
		objcache.RegisteredSharedSecretDeleteCallback(dv.GetVolID(), deleteRangerShare)
	case consts.ResourceReferenceTypeConfigMap:
		objcache.RegisterSharedConfigMapUpdateCallback(dv.GetVolID(), dv.GetSharedDataId(), updateRangerShare)
		objcache.RegisterSharedConfigMapDeleteCallback(dv.GetVolID(), deleteRangerShare)
	}

}

// createVolume create the directory for the csidriver volume.
// It returns the volume path or err if one occurs.
func (d *driver) createVolume(volID, targetPath string, refresh bool, volCtx map[string]string, cmShare *sharev1alpha1.SharedConfigMap, sShare *sharev1alpha1.SharedSecret, cap int64, volAccessType accessType) (*driverVolume, error) {
	if cmShare != nil && sShare != nil {
		return nil, fmt.Errorf("cannot store both SharedConfigMap and SharedSecret in a volume")
	}
	if cmShare == nil && sShare == nil {
		return nil, fmt.Errorf("have to provide either a SharedConfigMap or SharedSecret to a volume")
	}
	dv := d.getVolume(volID)
	if dv != nil {
		klog.V(0).Infof("createVolume: create call came in for volume %s that we have already created; returning previously created instance", volID)
		return dv, nil
	}
	anchorDir, bindDir := d.getVolumePath(volID, volCtx)
	switch volAccessType {
	case mountAccess:
		err := os.MkdirAll(anchorDir, 0777)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	vol := CreateDV(volID)
	vol.SetVolSize(cap)
	vol.SetVolPathAnchorDir(anchorDir)
	vol.SetVolPathBindMountDir(bindDir)
	vol.SetVolAccessType(volAccessType)
	vol.SetTargetPath(targetPath)
	vol.SetPodNamespace(podNamespace)
	vol.SetPodName(podName)
	vol.SetPodUID(podUID)
	vol.SetPodSA(podSA)
	vol.SetRefresh(refresh)
	switch {
	case cmShare != nil:
		vol.SetSharedDataKind(string(consts.ResourceReferenceTypeConfigMap))
		vol.SetSharedDataId(cmShare.Name)
	case sShare != nil:
		vol.SetSharedDataKind(string(consts.ResourceReferenceTypeSecret))
		vol.SetSharedDataId(sShare.Name)
	}

	return vol, nil
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

func (d *driver) innerDeleteVolume(top string) {
	// reminder, path is filepath.Join(DataRoot, [anchor-dir | bind-dir], volID, podNamespace, podName, podUID, podSA)
	// delete SA dir
	klog.V(4).Infof("innerDeleteVolume %s", top)
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

// deleteVolume deletes the directory for the csidriver volume.
func (d *driver) deleteVolume(volID string) error {
	klog.V(4).Infof("deleting csidriver volume: %s", volID)

	if dv := d.getVolume(volID); dv != nil {
		klog.V(4).Infof("found volume: %s", volID)
		os.RemoveAll(dv.GetTargetPath())
		remV(volID)
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

func (d *driver) loadVolsFromDisk() error {
	klog.V(2).Infof("loadVolsFromDisk")
	defer klog.V(2).Infof("loadVolsFromDisk exit")
	return filepath.Walk(d.volMapRoot, func(path string, info os.FileInfo, err error) error {
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
		fileName := filepath.Join(d.volMapRoot, info.Name())
		dataFile, oerr := os.Open(fileName)
		if oerr != nil {
			klog.V(0).Infof("loadVolsFromDisk error opening file %s: %s", fileName, oerr.Error())
			// continue to next file
			return nil
		}
		dataDecoder := json.NewDecoder(dataFile)
		dv := &driverVolume{}
		err = dataDecoder.Decode(dv)
		if err != nil {
			klog.V(0).Infof("loadVolsFromDisk error decoding file %s: %s", fileName, err.Error())
			// continue to next file
			return nil
		}
		if dv == nil {
			klog.V(0).Infof("loadVolsFromDisk nil but no error for file %s", fileName)
			// continue to next file
			return nil
		}
		dv.Lock = &sync.Mutex{}
		if filepath.Base(fileName) != dv.GetVolID() {
			klog.Warningf("loadVolsFromDisk file had vol id %s - corrupted !!!", dv.GetVolID())
			return nil
		}
		klog.V(2).Infof("loadVolsFromDisk storing with key %s dv %#v", dv.GetVolID(), dv)
		setDPV(dv.GetVolID(), dv)
		d.registerRangers(dv)

		return nil
	})
}

// Prune inspects all the volumes stored on disk and checks if their associated pods still exists.  If not, the volume
// file in question is deleted from disk.
func (d *driver) Prune(kubeClient kubernetes.Interface) {
	filesToPrune := map[string]driverVolume{}
	filepath.Walk(d.volMapRoot, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if err != nil {
			// continue to next file
			klog.V(5).Infof("Prune: for path %s given error %s", path, err.Error())
			return nil
		}
		if info.IsDir() {
			return nil
		}
		fileName := filepath.Join(d.volMapRoot, info.Name())
		dataFile, oerr := os.Open(fileName)
		if oerr != nil {
			klog.V(0).Infof("loadVolsFromDisk error opening file %s: %s", fileName, err.Error())
			// continue to next file
			return nil
		}
		dataDecoder := json.NewDecoder(dataFile)
		dv := &driverVolume{}
		err = dataDecoder.Decode(dv)
		if err != nil {
			klog.V(0).Infof("loadVolsFromDisk error decoding file %s: %s", fileName, err.Error())
			// continue to next file
			return nil
		}
		if dv == nil {
			klog.V(0).Infof("loadVolsFromDisk nil but no error for file %s", fileName)
			// continue to next file
			return nil
		}
		dv.Lock = &sync.Mutex{}
		_, err = kubeClient.CoreV1().Pods(dv.GetPodNamespace()).Get(context.TODO(), dv.GetPodName(), metav1.GetOptions{})
		if err != nil && kerrors.IsNotFound(err) {
			klog.V(2).Infof("pruner: dv %q: %s", fileName, err.Error())
			filesToPrune[fileName] = *dv
		}
		return nil
	})
	if len(filesToPrune) == 0 {
		return
	}
	// a bit paranoid, but not deleting files in the walk loop in case that can mess up filepath.Walk's iteration logic
	for file, dv := range filesToPrune {
		err := os.Remove(file)
		if err != nil {
			klog.Warningf("pruner: unable to prune file %q: %s", file, err.Error())
			continue
		}
		klog.V(2).Infof("pruner: removed volume file %q with missing pod from disk", file)
		if d.mounter != nil {
			err = d.mounter.Unmount(dv.GetVolPathAnchorDir())
			if err != nil {
				klog.Warningf("pruner: issue unmounting for volume %s mount id %s: %s", dv.GetVolID(), dv.GetVolPathAnchorDir(), err.Error())
			} else {
				klog.V(2).Infof("pruner: successfully unmounted volume %s mount id %s", dv.GetVolID(), dv.GetVolPathAnchorDir())
			}
		}
	}

}
