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
	"os"
	"path/filepath"

	objcache "github.com/openshift/csi-driver-projected-resource/pkg/cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/klog"
	utilexec "k8s.io/utils/exec"
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
	VolName       string     `json:"volName"`
	VolID         string     `json:"volID"`
	VolSize       int64      `json:"volSize"`
	VolPath       string     `json:"volPath"`
	VolAccessType accessType `json:"volAccessType"`
	TargetPath    string     `json:"targetPath"`
}

var (
	vendorVersion = "dev"

	hostPathVolumes map[string]hostPathVolume
)

const (
	// Directory where data for volumes and snapshots are persisted.
	// This can be ephemeral within the container or persisted if
	// backed by a Pod volume.
	DataRoot = "/csi-data-dir"
)

func init() {
	hostPathVolumes = map[string]hostPathVolume{}
}

type HostPathDriver interface {
	createHostpathVolume(volID, podNamespace, podName, podUID, podSA, targetPath string, cap int64, volAccessType accessType) (*hostPathVolume, error)
	deleteHostpathVolume(volID string) error
	getVolumePath(volID, podNamespace, podName, podUID, podSA string) string
	mapVolumeToPod(hpv *hostPathVolume) error
}

func NewHostPathDriver(root, driverName, nodeID, endpoint string, maxVolumesPerNode int64, version string) (*hostPath, error) {
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

	klog.Infof("Driver: %v ", driverName)
	klog.Infof("Version: %s", vendorVersion)

	return &hostPath{
		name:              driverName,
		version:           vendorVersion,
		nodeID:            nodeID,
		endpoint:          endpoint,
		maxVolumesPerNode: maxVolumesPerNode,
		root:              root,
	}, nil
}

func (hp *hostPath) Run() {
	// Create GRPC servers
	hp.ids = NewIdentityServer(hp.name, hp.version)
	hp.ns = NewNodeServer(hp)

	s := NewNonBlockingGRPCServer()
	s.Start(hp.endpoint, hp.ids, hp.ns)
	s.Wait()
}

func getVolumeByID(volumeID string) (hostPathVolume, error) {
	if hostPathVol, ok := hostPathVolumes[volumeID]; ok {
		return hostPathVol, nil
	}
	return hostPathVolume{}, fmt.Errorf("volume id %s does not exist in the volumes list", volumeID)
}

// getVolumePath returns the canonical path for hostpath volume
func (hp *hostPath) getVolumePath(volID, podNamespace, podName, podUID, podSA string) string {
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

func commonUpsertRanger(podPath string, key, value interface{}) bool {
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

func commonDeleteRanger(podPath string, key interface{}) bool {
	podFilePath := filepath.Join(podPath, fmt.Sprintf("%s", key))
	os.Remove(podFilePath)
	return true
}

func (hp *hostPath) mapVolumeToPod(hpv *hostPathVolume) error {
	podConfigMapsPath := filepath.Join(hpv.TargetPath, "configmaps")
	// for now, since os.MkdirAll does nothing and returns no error when the path already
	// exists, we have a common path for both create and update; but if we change the file
	// system interaction mechanism such that create and update are treated differently, we'll
	// need separate callbacks for each
	err := os.MkdirAll(podConfigMapsPath, 0777)
	if err != nil {
		return err
	}
	upsertRangerCM := func(key, value interface{}) bool {
		return commonUpsertRanger(podConfigMapsPath, key, value)
	}
	objcache.RegisterConfigMapUpsertCallback(hpv.VolID, upsertRangerCM)
	deleteRangerCM := func(key, value interface{}) bool {
		return commonDeleteRanger(podConfigMapsPath, key)
	}
	objcache.RegisterConfigMapDeleteCallback(hpv.VolID, deleteRangerCM)

	podSecretsPath := filepath.Join(hpv.TargetPath, "secrets")
	err = os.MkdirAll(podSecretsPath, 0777)
	if err != nil {
		return err
	}
	upsertRangerSec := func(key, value interface{}) bool {
		return commonUpsertRanger(podSecretsPath, key, value)
	}
	objcache.RegisterSecretUpsertCallback(hpv.VolID, upsertRangerSec)
	deleteRangerSec := func(key, value interface{}) bool {
		return commonDeleteRanger(podSecretsPath, key)
	}
	objcache.RegisterSecretDeleteCallback(hpv.VolID, deleteRangerSec)
	return nil
}

// createVolume create the directory for the hostpath volume.
// It returns the volume path or err if one occurs.
func (hp *hostPath) createHostpathVolume(volID, podNamespace, podName, podUID, podSA, targetPath string, cap int64, volAccessType accessType) (*hostPathVolume, error) {
	volPath := hp.getVolumePath(volID, podNamespace, podName, podUID, podSA)

	switch volAccessType {
	case mountAccess:
		err := os.MkdirAll(volPath, 0777)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	hostpathVol := hostPathVolume{
		VolID:         volID,
		VolSize:       cap,
		VolPath:       volPath,
		VolAccessType: volAccessType,
		TargetPath:    targetPath,
	}
	hostPathVolumes[volID] = hostpathVol
	return &hostpathVol, nil
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
	}
	objcache.UnregisterSecretUpsertCallback(volID)
	objcache.UnregisterSecretDeleteCallback(volID)
	objcache.UnregisterConfigMapDeleteCallback(volID)
	objcache.UnregisterConfigMapUpsertCallback(volID)
	return nil
}

// hostPathIsEmpty is a simple check to determine if the specified hostpath directory
// is empty or not.
func hostPathIsEmpty(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return true, fmt.Errorf("unable to open hostpath volume, error: %v", err)
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// loadFromVolume populates the given destPath with data from the srcVolumeID
func loadFromVolume(size int64, srcVolumeId, destPath string, mode accessType) error {
	hostPathVolume, ok := hostPathVolumes[srcVolumeId]
	if !ok {
		return status.Error(codes.NotFound, "source volumeId does not exist, are source/destination in the same storage class?")
	}
	if hostPathVolume.VolSize > size {
		return status.Errorf(codes.InvalidArgument, "volume %v size %v is greater than requested volume size %v", srcVolumeId, hostPathVolume.VolSize, size)
	}
	if mode != hostPathVolume.VolAccessType {
		return status.Errorf(codes.InvalidArgument, "volume %v mode is not compatible with requested mode", srcVolumeId)
	}

	switch mode {
	case mountAccess:
		return loadFromFilesystemVolume(hostPathVolume, destPath)
	default:
		return status.Errorf(codes.InvalidArgument, "unknown accessType: %d", mode)
	}
}

func loadFromFilesystemVolume(hostPathVolume hostPathVolume, destPath string) error {
	srcPath := hostPathVolume.VolPath
	isEmpty, err := hostPathIsEmpty(srcPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed verification check of source hostpath volume %v: %v", hostPathVolume.VolID, err)
	}

	// If the source hostpath volume is empty it's a noop and we just move along, otherwise the cp call will fail with a a file stat error DNE
	if !isEmpty {
		args := []string{"-a", srcPath + "/.", destPath + "/"}
		executor := utilexec.New()
		out, err := executor.Command("cp", args...).CombinedOutput()
		if err != nil {
			return status.Errorf(codes.Internal, "failed pre-populate data from volume %v: %v: %s", hostPathVolume.VolID, err, out)
		}
	}
	return nil
}
