package hostpath

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
type hostPathVolume struct {
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
	ReadOnly            bool       `json:"readOnly"`
	Refresh             bool       `json:"refresh"`
	// hpv's can be accessed/modified by both the sharedSecret/SharedConfigMap events and the configmap/secret events; to prevent data races
	// we serialize access to a given hpv with a per hpv mutex stored in this map; access to hpv fields should not
	// be done directly, but only by each field's getter and setter.  Getters and setters then leverage the per hpv
	// Lock objects stored in this map to prevent golang data races
	Lock *sync.Mutex `json:"-"` // we do not want this persisted to and from disk, so use of `json:"-"`
}

func CreateHPV(volID string) *hostPathVolume {
	hpv := &hostPathVolume{VolID: volID, Lock: &sync.Mutex{}}
	setHPV(volID, hpv)
	return hpv
}

func (hpv *hostPathVolume) GetVolID() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.VolID
}

func (hpv *hostPathVolume) GetVolName() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.VolName
}

func (hpv *hostPathVolume) GetVolSize() int64 {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.VolSize
}
func (hpv *hostPathVolume) GetVolPathAnchorDir() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.VolPathAnchorDir
}
func (hpv *hostPathVolume) GetVolPathBindMountDir() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.VolPathBindMountDir
}
func (hpv *hostPathVolume) GetVolAccessType() accessType {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.VolAccessType
}
func (hpv *hostPathVolume) GetTargetPath() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.TargetPath
}
func (hpv *hostPathVolume) GetSharedDataKind() consts.ResourceReferenceType {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return consts.ResourceReferenceType(hpv.SharedDataKind)
}
func (hpv *hostPathVolume) GetSharedDataId() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.SharedDataId
}
func (hpv *hostPathVolume) GetPodNamespace() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.PodNamespace
}
func (hpv *hostPathVolume) GetPodName() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.PodName
}
func (hpv *hostPathVolume) GetPodUID() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.PodUID
}
func (hpv *hostPathVolume) GetPodSA() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.PodSA
}
func (hpv *hostPathVolume) IsReadOnly() bool {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.ReadOnly
}

func (hpv *hostPathVolume) IsRefresh() bool {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.Refresh
}

func (hpv *hostPathVolume) SetVolName(volName string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.VolName = volName
}

func (hpv *hostPathVolume) SetVolSize(size int64) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.VolSize = size
}
func (hpv *hostPathVolume) SetVolPathAnchorDir(path string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.VolPathAnchorDir = path
}
func (hpv *hostPathVolume) SetVolPathBindMountDir(path string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.VolPathBindMountDir = path
}
func (hpv *hostPathVolume) SetVolAccessType(at accessType) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.VolAccessType = at
}
func (hpv *hostPathVolume) SetTargetPath(path string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.TargetPath = path
}
func (hpv *hostPathVolume) SetSharedDataKind(kind string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.SharedDataKind = kind
}
func (hpv *hostPathVolume) SetSharedDataId(id string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.SharedDataId = id
}
func (hpv *hostPathVolume) SetPodNamespace(namespace string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.PodNamespace = namespace
}
func (hpv *hostPathVolume) SetPodName(name string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.PodName = name
}
func (hpv *hostPathVolume) SetPodUID(uid string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.PodUID = uid
}
func (hpv *hostPathVolume) SetPodSA(sa string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.PodSA = sa
}
func (hpv *hostPathVolume) SetReadOnly(readOnly bool) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.ReadOnly = readOnly
}

func (hpv *hostPathVolume) SetRefresh(refresh bool) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.Refresh = refresh
}

func (hpv *hostPathVolume) StoreToDisk(volMapRoot string) error {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	klog.V(4).Infof("storeVolToDisk %s", hpv.VolID)
	defer klog.V(4).Infof("storeVolToDisk exit %s", hpv.VolID)

	f, terr := os.Open(volMapRoot)
	if terr != nil {
		// catch for unit tests
		return nil
	}
	defer f.Close()

	filePath := filepath.Join(volMapRoot, hpv.VolID)
	dataFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dataFile.Close()

	dataEncoder := json.NewEncoder(dataFile)
	return dataEncoder.Encode(hpv)
}
