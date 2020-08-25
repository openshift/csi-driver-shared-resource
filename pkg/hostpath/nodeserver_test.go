package hostpath

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"

	"k8s.io/utils/mount"
)

type fakeHostPath struct {
	volPath string
}

func (f *fakeHostPath) createHostpathVolume(volID, podNamespace, podName, podUID, podSA, targetPath string, cap int64, volAccessType accessType) (*hostPathVolume, error) {
	return &hostPathVolume{}, nil
}

func (f *fakeHostPath) deleteHostpathVolume(volID string) error {
	return nil
}

func (f *fakeHostPath) getVolumePath(volID, podNamespace, podName, podUID, podSA string) string {
	return f.volPath
}

func (f *fakeHostPath) mapVolumeToPod(hpv *hostPathVolume) error {
	return nil
}

func testNodeServer() (*nodeServer, string, error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		return nil, "", err
	}
	volPathTmpDir, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		return nil, "", err
	}
	ns := &nodeServer{
		nodeID:            "node1",
		maxVolumesPerNode: 0,
		mounter:           mount.NewFakeMounter([]mount.MountPoint{}),
		hp: &fakeHostPath{
			volPath: volPathTmpDir,
		},
	}
	return ns, tmpDir, nil
}

func getTestTargetPath(t *testing.T) string {
	dir, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return dir
}

func TestNodePublishVolume(t *testing.T) {
	tests := []struct {
		name              string
		nodePublishVolReq csi.NodePublishVolumeRequest
		expectedMsg       string
	}{
		{
			name:              "volume capabilities nil",
			nodePublishVolReq: csi.NodePublishVolumeRequest{},
			expectedMsg:       "Volume capability missing in request",
		},
		{
			name: "volume id is empty",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
			},
			expectedMsg: "Volume ID missing in request",
		},
		{
			name: "target path is empty",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
			},
			expectedMsg: "Target path missing in request",
		},
		{
			name: "volume context is not set",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
			},
			expectedMsg: "Volume attributes missing in request",
		},
		{
			name: "volume context missing required attributes",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext: map[string]string{
					"foo": "bar",
				},
			},
			expectedMsg: "Volume attributes missing required set for pod",
		},
		{
			name: "volume context is non-ephemeral",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext: map[string]string{
					CSIEphemeral:    "false",
					CSIPodName:      "name1",
					CSIPodNamespace: "namespace1",
					CSIPodUID:       "uid1",
					CSIPodSA:        "sa1",
				},
			},
			expectedMsg: "Non-ephemeral request made",
		},
		{
			name: "volume capabilities access is not mount type",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext: map[string]string{
					CSIEphemeral:    "true",
					CSIPodName:      "name1",
					CSIPodNamespace: "namespace1",
					CSIPodUID:       "uid1",
					CSIPodSA:        "sa1",
				},
			},
			expectedMsg: "only support mount access type",
		},
		{
			name: "inputs are OK",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:    "true",
					CSIPodName:      "name1",
					CSIPodNamespace: "namespace1",
					CSIPodUID:       "uid1",
					CSIPodSA:        "sa1",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.nodePublishVolReq.TargetPath != "" {
				defer os.RemoveAll(test.nodePublishVolReq.TargetPath)
			}
			ns, tmpDir, err := testNodeServer()
			if err != nil {
				t.Fatalf("unexpected err %s", err.Error())
			}
			defer os.RemoveAll(tmpDir)
			hp, _ := ns.hp.(*fakeHostPath)
			defer os.RemoveAll(hp.volPath)

			_, err = ns.NodePublishVolume(context.TODO(), &test.nodePublishVolReq)
			if len(test.expectedMsg) > 0 && err == nil || len(test.expectedMsg) == 0 && err != nil {
				t.Fatalf("expected err msg: %s, got: %+v", test.expectedMsg, err)
			}
			if len(test.expectedMsg) > 0 && !strings.Contains(err.Error(), test.expectedMsg) {
				t.Fatalf("instead of expected err msg containing %s got %s", test.expectedMsg, err.Error())
			}
			mnts, err := ns.mounter.List()
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}
			if len(test.expectedMsg) > 0 && len(mnts) != 0 {
				t.Fatalf("expected mount points to be 0")
			}
			if len(test.expectedMsg) == 0 && len(mnts) == 0 {
				t.Fatalf("expected mount points")
			}

		})
	}
}
