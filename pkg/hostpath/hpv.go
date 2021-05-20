package hostpath

import (
	"strconv"
	"sync"
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
	SharedDataKey       string     `json:"sharedDataKey"`
	SharedDataKind      string     `json:"sharedDataKind"`
	SharedDataId        string     `json:"sharedDataId"`
	ShareDataVersion    string     `json:"sharedDataVersion"`
	PodNamespace        string     `json:"podNamespace"`
	PodName             string     `json:"podName"`
	PodUID              string     `json:"podUID"`
	PodSA               string     `json:"podSA"`
	Allowed             bool       `json:"allowed"`
	ReadOnly            bool       `json:"readOnly"`
	// hpv's can be accessed/modified by both the share events and the configmap/secret events; to prevent data races
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
func (hpv *hostPathVolume) GetSharedDataKey() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.SharedDataKey
}
func (hpv *hostPathVolume) GetSharedDataKind() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.SharedDataKind
}
func (hpv *hostPathVolume) GetSharedDataId() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.SharedDataId
}
func (hpv *hostPathVolume) GetSharedDataVersion() string {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.ShareDataVersion
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
func (hpv *hostPathVolume) IsAllowed() bool {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.Allowed
}
func (hpv *hostPathVolume) IsReadOnly() bool {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	return hpv.ReadOnly
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
func (hpv *hostPathVolume) SetSharedDataKey(key string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.SharedDataKey = key
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
func (hpv *hostPathVolume) SetSharedDataVersion(version string) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.ShareDataVersion = version
}
func (hpv *hostPathVolume) CheckBeforeSetSharedDataVersion(version string) bool {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	newVersionInt, err := strconv.Atoi(version)
	if err != nil {
		return false
	}
	if len(hpv.ShareDataVersion) == 0 {
		hpv.ShareDataVersion = version
		return true
	}
	oldVersionInt, _ := strconv.Atoi(hpv.ShareDataVersion)
	if oldVersionInt >= newVersionInt {
		return false
	}
	hpv.ShareDataVersion = version
	return true
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
func (hpv *hostPathVolume) SetAllowed(allowed bool) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.Allowed = allowed
}
func (hpv *hostPathVolume) SetReadOnly(readOnly bool) {
	hpv.Lock.Lock()
	defer hpv.Lock.Unlock()
	hpv.ReadOnly = readOnly
}
