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
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
	objcache "github.com/openshift/csi-driver-projected-resource/pkg/cache"
	"github.com/openshift/csi-driver-projected-resource/pkg/client"

	"k8s.io/klog/v2"
)

const (
	kib int64 = 1024
	mib       = kib * 1024
	gib       = mib * 1024
	tib       = gib * 1024
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

type hostPathVolume struct {
	VolName        string     `json:"volName"`
	VolID          string     `json:"volID"`
	VolSize        int64      `json:"volSize"`
	VolPath        string     `json:"volPath"`
	VolAccessType  accessType `json:"volAccessType"`
	TargetPath     string     `json:"targetPath"`
	SharedDataKey  string     `json:"sharedDataKey"`
	SharedDataKind string     `json:"sharedDataKind"`
	SharedDataId   string     `json:"sharedDataId"`
	PodNamespace   string     `json:"podNamespace"`
	PodName        string     `json:"podName"`
	PodUID         string     `json:"podUID"`
	PodSA          string     `json:"podSA"`
	Allowed        bool       `json:"allowed"`
}

var (
	vendorVersion = "dev"

	hostPathVolumes map[string]*hostPathVolume

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
	VolumeMapFile = "volumemap.gob"
)

func init() {
	hostPathVolumes = map[string]*hostPathVolume{}
}

type HostPathDriver interface {
	createHostpathVolume(volID, targetPath string, volCtx map[string]string, share *sharev1alpha1.Share, cap int64, volAccessType accessType) (*hostPathVolume, error)
	deleteHostpathVolume(volID string) error
	getVolumePath(volID string, volCtx map[string]string) string
	mapVolumeToPod(hpv *hostPathVolume) error
}

func NewHostPathDriver(root, volMapRoot, driverName, nodeID, endpoint string, maxVolumesPerNode int64, version string) (*hostPath, error) {
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

	hp := &hostPath{
		name:              driverName,
		version:           vendorVersion,
		nodeID:            nodeID,
		endpoint:          endpoint,
		maxVolumesPerNode: maxVolumesPerNode,
		root:              root,
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
	hp.ns = NewNodeServer(hp)

	s := NewNonBlockingGRPCServer()
	s.Start(hp.endpoint, hp.ids, hp.ns)
	s.Wait()
}

// getVolumePath returns the canonical path for hostpath volume
func (hp *hostPath) getVolumePath(volID string, volCtx map[string]string) string {
	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	return filepath.Join(hp.root, volID, podNamespace, podName, podUID, podSA)
}

func createFile(path string, buf []byte) {
	file, err := os.Create(path)
	if err != nil {
		klog.Errorf("error creating file %s: %s", path, err.Error())
		return
	}
	defer file.Close()
	file.Write(buf)
}

func commonUpsertRanger(podPath, filter string, key, value interface{}) bool {
	if key != filter {
		return true
	}
	buf, err := json.MarshalIndent(value, "", "    ")
	if err != nil {
		klog.Errorf("error marshalling: %s", err.Error())
		return true
	}
	podFilePath := filepath.Join(podPath, fmt.Sprintf("%s", key))
	klog.V(4).Infof("create/update file %s", podFilePath)
	createFile(podFilePath, buf)
	return true
}

func commonDeleteRanger(podPath, filter string, key interface{}) bool {
	if key != filter {
		return true
	}
	podFilePath := filepath.Join(podPath, fmt.Sprintf("%s", key))
	os.Remove(podFilePath)
	return true
}

func shareDeleteRanger(hp *hostPath, key interface{}) bool {
	shareId := key.(string)
	targetPath := ""
	volID := ""
	for _, hpv := range hostPathVolumes {
		if hpv.SharedDataId == shareId {
			switch hpv.SharedDataKind {
			case "ConfigMap":
				targetPath = filepath.Join(hpv.TargetPath, "configmaps")
			case "Secret":
				targetPath = filepath.Join(hpv.TargetPath, "secrets")
			}
			volID = hpv.VolID
			// deleting the share effectively deletes permission to the
			// data so we set the allowed bit to false; this will have bearing
			// if the share is added again at a later date and the associated
			// pod in question is still up
			hpv.Allowed = false
			break
		}
	}
	if len(volID) > 0 && len(targetPath) > 0 {
		err := os.RemoveAll(targetPath)
		if err != nil {
			klog.Warningf("share %s vol %s target path %s delete error %s",
				shareId, volID, targetPath, err.Error())
		}
		// we just delete the associated data from the previously provisioned volume;
		// we don't delete the volume in case the share is added back
		storeVolMapToDisk()
	}
	return true
}

func shareUpdateRanger(key, value interface{}) bool {
	shareId := key.(string)
	share := value.(*sharev1alpha1.Share)
	klog.V(4).Infof("share update ranger id %s share name %s", shareId, share.Name)
	oldTargetPath := ""
	volID := ""
	change := false
	lostPermissions := false
	gainedPermissions := false
	hpv := &hostPathVolume{}
	for _, hpv = range hostPathVolumes {
		if hpv.SharedDataId == shareId {
			klog.V(4).Infof("share update ranger id %s found volume %s", shareId, hpv.VolID)
			a, err := client.ExecuteSAR(shareId, hpv.PodNamespace, hpv.PodName, hpv.PodSA)
			allowed := a && err == nil

			if allowed && !hpv.Allowed {
				klog.V(0).Infof("pod %s regained permissions for share %s",
					hpv.PodName, shareId)
				gainedPermissions = true
				hpv.Allowed = true
			}
			if !allowed && hpv.Allowed {
				klog.V(0).Infof("pod %s no longer has permission for share %s",
					hpv.PodName, shareId)
				lostPermissions = true
				hpv.Allowed = false
			}

			switch {
			case share.Spec.BackingResource.Kind != hpv.SharedDataKind:
				change = true
			case objcache.BuildKey(share.Spec.BackingResource.Namespace, share.Spec.BackingResource.Name) != hpv.SharedDataKey:
				change = true
			}
			if !change && !lostPermissions && !gainedPermissions {
				break
			}
			switch hpv.SharedDataKind {
			case "ConfigMap":
				oldTargetPath = filepath.Join(hpv.TargetPath, "configmaps")
			case "Secret":
				oldTargetPath = filepath.Join(hpv.TargetPath, "secrets")
			}
			volID = hpv.VolID
			break
		}
	}

	if lostPermissions {
		err := os.RemoveAll(oldTargetPath)
		if err != nil {
			klog.Warningf("share %s vol %s target path %s delete error %s",
				shareId, volID, oldTargetPath, err.Error())
		}
		objcache.UnregisterSecretUpsertCallback(volID)
		objcache.UnregisterSecretDeleteCallback(volID)
		objcache.UnregisterConfigMapDeleteCallback(volID)
		objcache.UnregisterConfigMapUpsertCallback(volID)
		storeVolMapToDisk()
		return true
	}

	if change {
		err := os.RemoveAll(oldTargetPath)
		if err != nil {
			klog.Warningf("share %s vol %s target path %s delete error %s",
				shareId, volID, oldTargetPath, err.Error())
		}
		objcache.UnregisterSecretUpsertCallback(volID)
		objcache.UnregisterSecretDeleteCallback(volID)
		objcache.UnregisterConfigMapDeleteCallback(volID)
		objcache.UnregisterConfigMapUpsertCallback(volID)

		hpv.SharedDataKind = share.Spec.BackingResource.Kind
		hpv.SharedDataKey = objcache.BuildKey(share.Spec.BackingResource.Namespace, share.Spec.BackingResource.Name)
		hpv.SharedDataId = share.Name

		mapBackingResourceToPod(hpv)
	}

	if gainedPermissions {
		mapBackingResourceToPod(hpv)
	}

	if change || gainedPermissions {
		storeVolMapToDisk()
	}

	return true
}

func mapBackingResourceToPod(hpv *hostPathVolume) error {
	// for now, since os.MkdirAll does nothing and returns no error when the path already
	// exists, we have a common path for both create and update; but if we change the file
	// system interaction mechanism such that create and update are treated differently, we'll
	// need separate callbacks for each
	switch strings.TrimSpace(hpv.SharedDataKind) {
	case "ConfigMap":
		podConfigMapsPath := filepath.Join(hpv.TargetPath, "configmaps")
		err := os.MkdirAll(podConfigMapsPath, 0777)
		if err != nil {
			return err
		}
		upsertRangerCM := func(key, value interface{}) bool {
			return commonUpsertRanger(podConfigMapsPath, hpv.SharedDataKey, key, value)
		}
		objcache.RegisterConfigMapUpsertCallback(hpv.VolID, upsertRangerCM)
		deleteRangerCM := func(key, value interface{}) bool {
			return commonDeleteRanger(podConfigMapsPath, hpv.SharedDataKey, key)
		}
		objcache.RegisterConfigMapDeleteCallback(hpv.VolID, deleteRangerCM)
	case "Secret":
		podSecretsPath := filepath.Join(hpv.TargetPath, "secrets")
		err := os.MkdirAll(podSecretsPath, 0777)
		if err != nil {
			return err
		}
		upsertRangerSec := func(key, value interface{}) bool {
			return commonUpsertRanger(podSecretsPath, hpv.SharedDataKey, key, value)
		}
		objcache.RegisterSecretUpsertCallback(hpv.VolID, upsertRangerSec)
		deleteRangerSec := func(key, value interface{}) bool {
			return commonDeleteRanger(podSecretsPath, hpv.SharedDataKey, key)
		}
		objcache.RegisterSecretDeleteCallback(hpv.VolID, deleteRangerSec)
	default:
		return fmt.Errorf("invalid share backing resource kind %s", hpv.SharedDataKind)
	}
	return nil
}

func (hp *hostPath) mapVolumeToPod(hpv *hostPathVolume) error {
	err := mapBackingResourceToPod(hpv)
	if err != nil {
		return err
	}
	deleteRangerShare := func(key, value interface{}) bool {
		return shareDeleteRanger(hp, key)
	}
	objcache.RegisterShareDeleteCallback(hpv.VolID, deleteRangerShare)
	updateRangerShare := func(key, value interface{}) bool {
		return shareUpdateRanger(key, value)
	}
	objcache.RegisterShareUpdateCallback(hpv.VolID, updateRangerShare)

	return nil
}

// createVolume create the directory for the hostpath volume.
// It returns the volume path or err if one occurs.
func (hp *hostPath) createHostpathVolume(volID, targetPath string, volCtx map[string]string, share *sharev1alpha1.Share, cap int64, volAccessType accessType) (*hostPathVolume, error) {
	volPath := hp.getVolumePath(volID, volCtx)
	switch volAccessType {
	case mountAccess:
		err := os.MkdirAll(volPath, 0777)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	podNamespace, podName, podUID, podSA := getPodDetails(volCtx)
	hostpathVol := &hostPathVolume{
		VolID:          volID,
		VolSize:        cap,
		VolPath:        volPath,
		VolAccessType:  volAccessType,
		TargetPath:     targetPath,
		PodNamespace:   podNamespace,
		PodName:        podName,
		PodUID:         podUID,
		PodSA:          podSA,
		SharedDataKind: share.Spec.BackingResource.Kind,
		SharedDataKey:  objcache.BuildKey(share.Spec.BackingResource.Namespace, share.Spec.BackingResource.Name),
		SharedDataId:   share.Name,
		Allowed:        true,
	}
	hostPathVolumes[volID] = hostpathVol
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

// deleteVolume deletes the directory for the hostpath volume.
func (hp *hostPath) deleteHostpathVolume(volID string) error {
	klog.V(4).Infof("deleting hostpath volume: %s", volID)

	hpv, ok := hostPathVolumes[volID]
	if ok {
		// reminder, path is filepath.Join(DataRoot, volID, podNamespace, podName, podUID, podSA)
		// delete SA dir
		err := os.RemoveAll(hpv.VolPath)
		if err != nil {
			klog.Warningf("error deleting %s: %s", hpv.VolPath, err.Error())
		}
		uidPath := filepath.Dir(hpv.VolPath)
		deleteIfEmpty(uidPath)
		namePath := filepath.Dir(uidPath)
		deleteIfEmpty(namePath)
		namespacePath := filepath.Dir(namePath)
		deleteIfEmpty(namespacePath)
		volidPath := filepath.Dir(namespacePath)
		deleteIfEmpty(volidPath)
		delete(hostPathVolumes, volID)
		storeVolMapToDisk()
	}
	objcache.UnregisterSecretUpsertCallback(volID)
	objcache.UnregisterSecretDeleteCallback(volID)
	objcache.UnregisterConfigMapDeleteCallback(volID)
	objcache.UnregisterConfigMapUpsertCallback(volID)
	objcache.UnregisterShareDeleteCallback(volID)
	objcache.UnregisterShareUpdateCallback(volID)
	return nil
}

func storeVolMapToDisk() error {
	fileWriteLock.Lock()
	defer fileWriteLock.Unlock()
	klog.V(4).Info("storeVolMapToDisk")
	dataFile, err := os.Create(volMapOnDiskPath)
	if err != nil {
		klog.Warningf("error creating map file: %s", err.Error())
		return err
	}
	defer dataFile.Close()
	dataEncoder := gob.NewEncoder(dataFile)
	mapCopy := map[string]hostPathVolume{}
	for k, v := range hostPathVolumes {
		mapCopy[k] = *v
	}
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
	dataDecoder := gob.NewDecoder(dataFile)
	err = dataDecoder.Decode(&mapCopy)
	if err != nil {
		klog.Warningf("error decoding map file: %s", err.Error())
		return err
	}
	hostPathVolumes = map[string]*hostPathVolume{}
	for k, v := range mapCopy {
		klog.V(4).Infof("loadVolMapFromDisk looking at volume %s hpv %#v", k, v)
		pod, err := client.GetPod(v.PodNamespace, v.PodName)
		if err != nil {
			klog.V(2).Infof("loadVolMapFromDisk could not find pod %s:%s so dropping: %s",
				v.PodNamespace, v.PodName, err.Error())
			continue
		}
		if string(pod.UID) != v.PodUID {
			klog.V(2).Infof("loadVolMapFromDisk found pod %s:%s but UIDs do no match so dropping: %s vs %s",
				v.PodNamespace, v.PodName, string(pod.UID), v.PodUID)
			continue
		}
		hostPathVolumes[k] = &v
		err = hp.mapVolumeToPod(&v)
		if err != nil {
			klog.Warningf("loadVolMapFromDisk error mapping volume %s to shares: %s", k, err.Error())
		}
	}
	return nil
}
