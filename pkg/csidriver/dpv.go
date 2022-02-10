package csidriver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/klog/v2"

	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
)

// NOTE / TODO: the fields in this struct need to start with a capital letter since we are
// externalizing / storing to disk, unless there is someway to get the golang encoding
// logic to use our getters/setters
type driverVolume struct {
	VolID               string     `json:"volID"`
	VolName             string     `json:"volName"`
	VolSize             int64      `json:"volSize"`
	VolPathAnchorDir    string     `json:"volPathAnchorDir"`
	VolPathBindMountDir string     `json:"volPathBindMountDir"`
	VolAccessType       accessType `json:"volAccessType"`
	TargetPath          string     `json:"targetPath"`
	SharedDataKind      string     `json:"sharedDataKind"`
	SharedDataId        string     `json:"sharedDataId"`
	PodNamespace        string     `json:"podNamespace"`
	PodName             string     `json:"podName"`
	PodUID              string     `json:"podUID"`
	PodSA               string     `json:"podSA"`
	Refresh             bool       `json:"refresh"`
	// dpv's can be accessed/modified by both the sharedSecret/SharedConfigMap events and the configmap/secret events; to prevent data races
	// we serialize access to a given dpv with a per dpv mutex stored in this map; access to dpv fields should not
	// be done directly, but only by each field's getter and setter.  Getters and setters then leverage the per dpv
	// Lock objects stored in this map to prevent golang data races
	Lock *sync.Mutex `json:"-"` // we do not want this persisted to and from disk, so use of `json:"-"`
}

func CreateDV(volID string) *driverVolume {
	dpv := &driverVolume{VolID: volID, Lock: &sync.Mutex{}}
	setDPV(volID, dpv)
	return dpv
}

func (dpv *driverVolume) GetVolID() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.VolID
}

func (dpv *driverVolume) GetVolName() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.VolName
}

func (dpv *driverVolume) GetVolSize() int64 {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.VolSize
}
func (dpv *driverVolume) GetVolPathAnchorDir() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.VolPathAnchorDir
}
func (dpv *driverVolume) GetVolPathBindMountDir() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.VolPathBindMountDir
}
func (dpv *driverVolume) GetVolAccessType() accessType {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.VolAccessType
}
func (dpv *driverVolume) GetTargetPath() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.TargetPath
}
func (dpv *driverVolume) GetSharedDataKind() consts.ResourceReferenceType {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return consts.ResourceReferenceType(dpv.SharedDataKind)
}
func (dpv *driverVolume) GetSharedDataId() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.SharedDataId
}
func (dpv *driverVolume) GetPodNamespace() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.PodNamespace
}
func (dpv *driverVolume) GetPodName() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.PodName
}
func (dpv *driverVolume) GetPodUID() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.PodUID
}
func (dpv *driverVolume) GetPodSA() string {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.PodSA
}
func (dpv *driverVolume) IsRefresh() bool {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	return dpv.Refresh
}

func (dpv *driverVolume) SetVolName(volName string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.VolName = volName
}

func (dpv *driverVolume) SetVolSize(size int64) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.VolSize = size
}
func (dpv *driverVolume) SetVolPathAnchorDir(path string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.VolPathAnchorDir = path
}
func (dpv *driverVolume) SetVolPathBindMountDir(path string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.VolPathBindMountDir = path
}
func (dpv *driverVolume) SetVolAccessType(at accessType) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.VolAccessType = at
}
func (dpv *driverVolume) SetTargetPath(path string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.TargetPath = path
}
func (dpv *driverVolume) SetSharedDataKind(kind string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.SharedDataKind = kind
}
func (dpv *driverVolume) SetSharedDataId(id string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.SharedDataId = id
}
func (dpv *driverVolume) SetPodNamespace(namespace string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.PodNamespace = namespace
}
func (dpv *driverVolume) SetPodName(name string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.PodName = name
}
func (dpv *driverVolume) SetPodUID(uid string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.PodUID = uid
}
func (dpv *driverVolume) SetPodSA(sa string) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.PodSA = sa
}
func (dpv *driverVolume) SetRefresh(refresh bool) {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	dpv.Refresh = refresh
}

func (dpv *driverVolume) StoreToDisk(volMapRoot string) error {
	dpv.Lock.Lock()
	defer dpv.Lock.Unlock()
	klog.V(4).Infof("storeVolToDisk %s", dpv.VolID)
	defer klog.V(4).Infof("storeVolToDisk exit %s", dpv.VolID)

	f, terr := os.Open(volMapRoot)
	if terr != nil {
		// catch for unit tests
		return nil
	}
	defer f.Close()

	filePath := filepath.Join(volMapRoot, dpv.VolID)
	dataFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dataFile.Close()

	dataEncoder := json.NewEncoder(dataFile)
	return dataEncoder.Encode(dpv)
}
