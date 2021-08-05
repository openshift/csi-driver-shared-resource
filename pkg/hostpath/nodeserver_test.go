package hostpath

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	sharev1alpha1 "github.com/openshift/csi-driver-shared-resource/pkg/api/projectedresource/v1alpha1"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"golang.org/x/net/context"

	authorizationv1 "k8s.io/api/authorization/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubetesting "k8s.io/client-go/testing"
	"k8s.io/utils/mount"
)

type fakeShareLister struct {
	share *sharev1alpha1.Share
}

func (f *fakeShareLister) List(selector labels.Selector) (ret []*sharev1alpha1.Share, err error) {
	if f.share == nil {
		return []*sharev1alpha1.Share{}, nil
	}
	return []*sharev1alpha1.Share{f.share}, nil
}

func (f *fakeShareLister) Get(name string) (*sharev1alpha1.Share, error) {
	if f.share == nil {
		return nil, kerrors.NewNotFound(schema.GroupResource{}, name)
	}
	return f.share, nil
}

func testNodeServer(testName string) (*nodeServer, string, string, error) {
	if strings.Contains(testName, "/") {
		testName = strings.Split(testName, "/")[0]
	}
	hp, tmpDir, volPathTmpDir, err := testHostPathDriver(testName)
	if err != nil {
		return nil, "", "", err
	}
	ns := &nodeServer{
		nodeID:            "node1",
		maxVolumesPerNode: 0,
		mounter:           mount.NewFakeMounter([]mount.MountPoint{}),
		readOnlyMounter:   &WriteOnceReadMany{},
		readWriteMounter:  &ReadWriteMany{},
		hp:                hp,
	}
	return ns, tmpDir, volPathTmpDir, nil
}

func getTestTargetPath(t *testing.T) string {
	dir, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return dir
}

func TestNodePublishVolume(t *testing.T) {
	var acceptReactorFunc, denyReactorFunc fakekubetesting.ReactionFunc
	acceptReactorFunc = func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	denyReactorFunc = func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: false}}, nil
	}
	validShare := &sharev1alpha1.Share{
		ObjectMeta: metav1.ObjectMeta{
			Name: "share1",
		},
		Spec: sharev1alpha1.ShareSpec{
			BackingResource: sharev1alpha1.BackingResource{
				Kind:       "Secret",
				APIVersion: "v1",
				Name:       "cool-secret",
				Namespace:  "cool-secret-namespace",
			},
			Description: "",
		},
		Status: sharev1alpha1.ShareStatus{},
	}

	tests := []struct {
		name              string
		nodePublishVolReq csi.NodePublishVolumeRequest
		expectedMsg       string
		share             *sharev1alpha1.Share
		reactor           fakekubetesting.ReactionFunc
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
			name: "missing share key",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
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
			expectedMsg: "the csi driver reference is missing the volumeAttribute 'share'",
		},
		{
			name: "missing share",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:              "true",
					CSIPodName:                "name1",
					CSIPodNamespace:           "namespace1",
					CSIPodUID:                 "uid1",
					CSIPodSA:                  "sa1",
					ProjectedResourceShareKey: "share1",
				},
			},
			expectedMsg: "the csi driver volumeAttribute 'share' reference had an error",
		},
		{
			name: "bad backing resource kind",
			share: &sharev1alpha1.Share{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.ShareSpec{
					BackingResource: sharev1alpha1.BackingResource{
						Kind: "BadKind",
					},
					Description: "",
				},
				Status: sharev1alpha1.ShareStatus{},
			},
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:              "true",
					CSIPodName:                "name1",
					CSIPodNamespace:           "namespace1",
					CSIPodUID:                 "uid1",
					CSIPodSA:                  "sa1",
					ProjectedResourceShareKey: "share1",
				},
			},
			expectedMsg: "has an invalid backing resource kind",
		},
		{
			name: "bad backing resource namespace",
			share: &sharev1alpha1.Share{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.ShareSpec{
					BackingResource: sharev1alpha1.BackingResource{
						Kind: "ConfigMap",
						Name: "configmap1",
					},
					Description: "",
				},
				Status: sharev1alpha1.ShareStatus{},
			},
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:              "true",
					CSIPodName:                "name1",
					CSIPodNamespace:           "namespace1",
					CSIPodUID:                 "uid1",
					CSIPodSA:                  "sa1",
					ProjectedResourceShareKey: "share1",
				},
			},
			expectedMsg: "backing resource namespace needs to be set",
		},
		{
			name: "bad backing resource name",
			share: &sharev1alpha1.Share{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.ShareSpec{
					BackingResource: sharev1alpha1.BackingResource{
						Kind:      "ConfigMap",
						Namespace: "namespace1",
					},
					Description: "",
				},
				Status: sharev1alpha1.ShareStatus{},
			},
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:              "true",
					CSIPodName:                "name1",
					CSIPodNamespace:           "namespace1",
					CSIPodUID:                 "uid1",
					CSIPodSA:                  "sa1",
					ProjectedResourceShareKey: "share1",
				},
			},
			expectedMsg: "backing resource name needs to be set",
		},
		{
			name:    "sar fails",
			share:   validShare,
			reactor: denyReactorFunc,
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:              "true",
					CSIPodName:                "name1",
					CSIPodNamespace:           "namespace1",
					CSIPodUID:                 "uid1",
					CSIPodSA:                  "sa1",
					ProjectedResourceShareKey: "share1",
				},
			},
			expectedMsg: "PermissionDenied",
		},
		{
			name:    "inputs are OK",
			share:   validShare,
			reactor: acceptReactorFunc,
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				Readonly:   true,
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:              "true",
					CSIPodName:                "name1",
					CSIPodNamespace:           "namespace1",
					CSIPodUID:                 "uid1",
					CSIPodSA:                  "sa1",
					ProjectedResourceShareKey: "share1",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.nodePublishVolReq.TargetPath != "" {
				defer os.RemoveAll(test.nodePublishVolReq.TargetPath)
			}
			ns, tmpDir, volPath, err := testNodeServer(t.Name())
			if err != nil {
				t.Fatalf("unexpected err %s", err.Error())
			}
			defer os.RemoveAll(tmpDir)
			defer os.RemoveAll(volPath)

			shareLister := &fakeShareLister{
				share: test.share,
			}
			client.SetSharesLister(shareLister)

			if test.reactor != nil {
				sarClient := fakekubeclientset.NewSimpleClientset()
				sarClient.PrependReactor("create", "subjectaccessreviews", test.reactor)
				client.SetClient(sarClient)
			}

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
