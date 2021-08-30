package hostpath

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
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

	storagev1alpha1 "github.com/openshift/api/storage/v1alpha1"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

type fakeShareLister struct {
	share *storagev1alpha1.SharedResource
}

func (f *fakeShareLister) List(selector labels.Selector) (ret []*storagev1alpha1.SharedResource, err error) {
	if f.share == nil {
		return []*storagev1alpha1.SharedResource{}, nil
	}
	return []*storagev1alpha1.SharedResource{f.share}, nil
}

func (f *fakeShareLister) Get(name string) (*storagev1alpha1.SharedResource, error) {
	if f.share == nil {
		return nil, kerrors.NewNotFound(schema.GroupResource{}, name)
	}
	return f.share, nil
}

func testNodeServer(testName string) (*nodeServer, string, string, error) {
	if strings.Contains(testName, "/") {
		testName = strings.Split(testName, "/")[0]
	}
	hp, tmpDir, volPathTmpDir, err := testHostPathDriver(testName, nil)
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
	validShare := &storagev1alpha1.SharedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "share1",
		},
		Spec: storagev1alpha1.SharedResourceSpec{
			Resource: storagev1alpha1.ResourceReference{
				Type: storagev1alpha1.ResourceReferenceTypeSecret,
				Secret: &storagev1alpha1.ResourceReferenceSecret{
					Name:      "cool-secret",
					Namespace: "cool-secret-namespace",
				},
			},
			Description: "",
		},
		Status: storagev1alpha1.SharedResourceStatus{},
	}

	tests := []struct {
		name              string
		nodePublishVolReq csi.NodePublishVolumeRequest
		expectedMsg       string
		share             *storagev1alpha1.SharedResource
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
			expectedMsg: fmt.Sprintf("the csi driver reference is missing the volumeAttribute '%s'", SharedResourceShareKey),
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
					CSIEphemeral:           "true",
					CSIPodName:             "name1",
					CSIPodNamespace:        "namespace1",
					CSIPodUID:              "uid1",
					CSIPodSA:               "sa1",
					SharedResourceShareKey: "share1",
				},
			},
			expectedMsg: fmt.Sprintf("the csi driver volumeAttribute '%s' reference had an error", SharedResourceShareKey),
		},
		{
			name: "bad backing resource namespace",
			share: &storagev1alpha1.SharedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: storagev1alpha1.SharedResourceSpec{
					Resource: storagev1alpha1.ResourceReference{
						Type: storagev1alpha1.ResourceReferenceTypeConfigMap,
						ConfigMap: &storagev1alpha1.ResourceReferenceConfigMap{
							Name: "configmap1",
						},
					},
					Description: "",
				},
				Status: storagev1alpha1.SharedResourceStatus{},
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
					CSIEphemeral:           "true",
					CSIPodName:             "name1",
					CSIPodNamespace:        "namespace1",
					CSIPodUID:              "uid1",
					CSIPodSA:               "sa1",
					SharedResourceShareKey: "share1",
				},
			},
			expectedMsg: "must have a namespace set",
		},
		{
			name: "bad backing resource name",
			share: &storagev1alpha1.SharedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: storagev1alpha1.SharedResourceSpec{
					Resource: storagev1alpha1.ResourceReference{
						Type: storagev1alpha1.ResourceReferenceTypeConfigMap,
						ConfigMap: &storagev1alpha1.ResourceReferenceConfigMap{
							Namespace: "namespace1",
						},
					},
					Description: "",
				},
				Status: storagev1alpha1.SharedResourceStatus{},
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
					CSIEphemeral:           "true",
					CSIPodName:             "name1",
					CSIPodNamespace:        "namespace1",
					CSIPodUID:              "uid1",
					CSIPodSA:               "sa1",
					SharedResourceShareKey: "share1",
				},
			},
			expectedMsg: "must have a name set",
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
					CSIEphemeral:           "true",
					CSIPodName:             "name1",
					CSIPodNamespace:        "namespace1",
					CSIPodUID:              "uid1",
					CSIPodSA:               "sa1",
					SharedResourceShareKey: "share1",
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
					CSIEphemeral:           "true",
					CSIPodName:             "name1",
					CSIPodNamespace:        "namespace1",
					CSIPodUID:              "uid1",
					CSIPodSA:               "sa1",
					SharedResourceShareKey: "share1",
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
