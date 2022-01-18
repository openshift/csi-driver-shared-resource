package hostpath

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
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

type fakeSharedSecretLister struct {
	sShare *sharev1alpha1.SharedSecret
}

func (f *fakeSharedSecretLister) List(selector labels.Selector) (ret []*sharev1alpha1.SharedSecret, err error) {
	if f.sShare == nil {
		return []*sharev1alpha1.SharedSecret{}, nil
	}
	return []*sharev1alpha1.SharedSecret{f.sShare}, nil
}

func (f *fakeSharedSecretLister) Get(name string) (*sharev1alpha1.SharedSecret, error) {
	if f.sShare == nil {
		return nil, kerrors.NewNotFound(schema.GroupResource{}, name)
	}
	return f.sShare, nil
}

type fakeSharedConfigMapLister struct {
	cmShare *sharev1alpha1.SharedConfigMap
}

func (f *fakeSharedConfigMapLister) List(selector labels.Selector) (ret []*sharev1alpha1.SharedConfigMap, err error) {
	if f.cmShare == nil {
		return []*sharev1alpha1.SharedConfigMap{}, nil
	}
	return []*sharev1alpha1.SharedConfigMap{f.cmShare}, nil
}

func (f *fakeSharedConfigMapLister) Get(name string) (*sharev1alpha1.SharedConfigMap, error) {
	if f.cmShare == nil {
		return nil, kerrors.NewNotFound(schema.GroupResource{}, name)
	}
	return f.cmShare, nil
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
	validSharedSecret := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "share1",
		},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{
				Name:      "cool-secret",
				Namespace: "cool-secret-namespace",
			},
			Description: "",
		},
		Status: sharev1alpha1.SharedSecretStatus{},
	}
	validSharedConfigMap := &sharev1alpha1.SharedConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "share1",
		},
		Spec: sharev1alpha1.SharedConfigMapSpec{
			ConfigMapRef: sharev1alpha1.SharedConfigMapReference{
				Name:      "cool-configmap",
				Namespace: "cool-configmap-namespace",
			},
		},
	}

	tests := []struct {
		name              string
		nodePublishVolReq csi.NodePublishVolumeRequest
		expectedMsg       string
		secretShare       *sharev1alpha1.SharedSecret
		cmShare           *sharev1alpha1.SharedConfigMap
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
			name: "missing sharedSecret/sharedConfigMap key",
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
			expectedMsg: "the csi driver reference is missing the volumeAttribute \"sharedSecret\" and \"sharedConfigMap\"",
		},
		{
			name: "both sharedSecret and sharedConfigMap",
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
					SharedSecretShareKey:    "share1",
				},
			},
			expectedMsg: "a single volume cannot support both a SharedConfigMap reference \"share1\" and SharedSecret reference \"share1\"",
		},
		{
			name: "bad sharedSecret backing resource namespace",
			secretShare: &sharev1alpha1.SharedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.SharedSecretSpec{
					SecretRef: sharev1alpha1.SharedSecretReference{
						Name: "secret1",
					},
					Description: "",
				},
				Status: sharev1alpha1.SharedSecretStatus{},
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
					CSIEphemeral:         "true",
					CSIPodName:           "name1",
					CSIPodNamespace:      "namespace1",
					CSIPodUID:            "uid1",
					CSIPodSA:             "sa1",
					SharedSecretShareKey: "share1",
				},
			},
			expectedMsg: "the SharedSecret \"share1\" backing resource namespace needs to be set",
		},
		{
			name: "bad sharedConfigMap backing resource namespace",
			cmShare: &sharev1alpha1.SharedConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.SharedConfigMapSpec{
					ConfigMapRef: sharev1alpha1.SharedConfigMapReference{
						Name: "cm1",
					},
					Description: "",
				},
				Status: sharev1alpha1.SharedConfigMapStatus{},
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
				},
			},
			expectedMsg: "the SharedConfigMap \"share1\" backing resource namespace needs to be set",
		},
		{
			name: "bad sharedSecret backing resource name",
			secretShare: &sharev1alpha1.SharedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.SharedSecretSpec{
					SecretRef: sharev1alpha1.SharedSecretReference{
						Namespace: "secret1",
					},
					Description: "",
				},
				Status: sharev1alpha1.SharedSecretStatus{},
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
					CSIEphemeral:         "true",
					CSIPodName:           "name1",
					CSIPodNamespace:      "namespace1",
					CSIPodUID:            "uid1",
					CSIPodSA:             "sa1",
					SharedSecretShareKey: "share1",
				},
			},
			expectedMsg: "the SharedSecret \"share1\" backing resource name needs to be set",
		},
		{
			name: "bad sharedConfigMap backing resource name",
			cmShare: &sharev1alpha1.SharedConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share1",
				},
				Spec: sharev1alpha1.SharedConfigMapSpec{
					ConfigMapRef: sharev1alpha1.SharedConfigMapReference{
						Namespace: "cm1",
					},
					Description: "",
				},
				Status: sharev1alpha1.SharedConfigMapStatus{},
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
				},
			},
			expectedMsg: "the SharedConfigMap \"share1\" backing resource name needs to be set",
		},
		{
			name:        "sar fails secret",
			secretShare: validSharedSecret,
			reactor:     denyReactorFunc,
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
					CSIEphemeral:         "true",
					CSIPodName:           "name1",
					CSIPodNamespace:      "namespace1",
					CSIPodUID:            "uid1",
					CSIPodSA:             "sa1",
					SharedSecretShareKey: "share1",
				},
			},
			expectedMsg: "PermissionDenied",
		},
		{
			name:    "sar fails configmap",
			cmShare: validSharedConfigMap,
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
				},
			},
			expectedMsg: "PermissionDenied",
		},
		{
			name:    "read only flag not set to true",
			cmShare: validSharedConfigMap,
			reactor: acceptReactorFunc,
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeId:   "testvolid1",
				TargetPath: getTestTargetPath(t),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
				},
				VolumeContext: map[string]string{
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
				},
			},
			expectedMsg: "The Shared Resource CSI driver requires all volume requests to set read-only to",
		},
		{
			name:    "inputs are OK for configmap",
			cmShare: validSharedConfigMap,
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
				},
			},
		},
		{
			name:        "inputs are OK for secret",
			secretShare: validSharedSecret,
			reactor:     acceptReactorFunc,
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
					CSIEphemeral:         "true",
					CSIPodName:           "name1",
					CSIPodNamespace:      "namespace1",
					CSIPodUID:            "uid1",
					CSIPodSA:             "sa1",
					SharedSecretShareKey: "share1",
				},
			},
		},
		{
			name:        "inputs are OK for secret, no refresh",
			secretShare: validSharedSecret,
			reactor:     acceptReactorFunc,
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
					CSIEphemeral:         "true",
					CSIPodName:           "name1",
					CSIPodNamespace:      "namespace1",
					CSIPodUID:            "uid1",
					CSIPodSA:             "sa1",
					SharedSecretShareKey: "share1",
					RefreshResource:      "false",
				},
			},
		},
		{
			name:    "inputs are OK for configmap, no refresh",
			cmShare: validSharedConfigMap,
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
					RefreshResource:         "false",
				},
			},
		},
		{
			name:        "inputs are OK for secret, no refresh",
			secretShare: validSharedSecret,
			reactor:     acceptReactorFunc,
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
					CSIEphemeral:         "true",
					CSIPodName:           "name1",
					CSIPodNamespace:      "namespace1",
					CSIPodUID:            "uid1",
					CSIPodSA:             "sa1",
					SharedSecretShareKey: "share1",
					RefreshResource:      "false",
				},
			},
		},
		{
			name:    "inputs are OK for configmap, no refresh",
			cmShare: validSharedConfigMap,
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
					CSIEphemeral:            "true",
					CSIPodName:              "name1",
					CSIPodNamespace:         "namespace1",
					CSIPodUID:               "uid1",
					CSIPodSA:                "sa1",
					SharedConfigMapShareKey: "share1",
					RefreshResource:         "false",
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

			secretShareLister := &fakeSharedSecretLister{
				sShare: test.secretShare,
			}
			client.SetSharedSecretsLister(secretShareLister)
			cmShareLister := &fakeSharedConfigMapLister{
				cmShare: test.cmShare,
			}
			client.SetSharedConfigMapsLister(cmShareLister)

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
