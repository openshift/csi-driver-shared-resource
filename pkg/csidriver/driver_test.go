/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubetesting "k8s.io/client-go/testing"
	"k8s.io/utils/mount"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	fakeshareclientset "github.com/openshift/client-go/sharedresource/clientset/versioned/fake"
	shareinformer "github.com/openshift/client-go/sharedresource/informers/externalversions"
	"github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

const (
	secretkey1      = "secretkey1"
	secretvalue1    = "secretvalue1"
	secretkey2      = "secretkey2"
	secretvalue2    = "secretvalue2"
	configmapkey1   = "configmapkey1"
	configmapvalue1 = "configmapvalue1"
)

// the actual Mounter from k8s/util requires sudo privileges to run, so for the pruner related tests we create a fake
// mounter since the k8s code does not provide a fake mounter
type fakeMounter struct {
}

func (f *fakeMounter) Mount(source string, target string, fstype string, options []string) error {
	return nil
}

func (f *fakeMounter) MountSensitive(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	return nil
}

func (f *fakeMounter) Unmount(target string) error {
	return nil
}

func (f *fakeMounter) List() ([]mount.MountPoint, error) {
	return nil, nil
}

func (f *fakeMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	return false, nil
}

func (f *fakeMounter) GetMountRefs(pathname string) ([]string, error) {
	return nil, nil
}

func testDriver(testName string, kubeClient kubernetes.Interface) (CSIDriver, string, string, error) {
	tmpDir1, err := ioutil.TempDir(os.TempDir(), testName)
	if err != nil {
		return nil, "", "", err
	}
	tmpDir2, err := ioutil.TempDir(os.TempDir(), testName)
	if err != nil {
		return nil, "", "", err
	}
	if kubeClient != nil {
		client.SetClient(kubeClient)
	}
	d, err := NewCSIDriver(tmpDir1, tmpDir2, testName, "nodeID1", "endpoint1", 0, "version1", &fakeMounter{})
	return d, tmpDir1, tmpDir2, err
}

func seedVolumeContext() map[string]string {
	volCtx := map[string]string{
		CSIPodName:      "podName",
		CSIPodNamespace: "podNamespace",
		CSIPodSA:        "podSA",
		CSIPodUID:       "podUID",
	}
	return volCtx
}

func TestCreateVolumeBadAccessType(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	volCtx := seedVolumeContext()
	_, err = d.createVolume(t.Name(), "", true, volCtx, &sharev1alpha1.SharedConfigMap{}, nil, 0, mountAccess+1)
	if err == nil {
		t.Fatalf("err nil unexpectedly")
	}
	if !strings.Contains(err.Error(), "unsupported access type") {
		t.Fatalf("unexpected err %s", err.Error())
	}
}

func TestCreateDeleteConfigMap(t *testing.T) {
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	d, dir1, dir2, err := testDriver(t.Name(), k8sClient)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	cm, searchPath := primeConfigMapVolume(t, d, targetPath, nil, k8sClient, shareClient)
	_, foundConfigMap := findSharedItems(t, searchPath)
	if !foundConfigMap {
		t.Fatalf("did not find configmap in mount path ")
	}
	cache.DelConfigMap(cm)
	_, foundConfigMap = findSharedItems(t, searchPath)
	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
	// clear out dv for next run
	d.deleteVolume(t.Name())

}

func TestCreateDeleteSecret(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	k8sClient := fakekubeclientset.NewSimpleClientset()
	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	client.SetShareClient(shareClient)
	secret, searchPath := primeSecretVolume(t, d, targetPath, nil, k8sClient, shareClient)
	foundSecret, _ := findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("did not find secret in mount path")
	}
	cache.DelSecret(secret)
	foundSecret, _ = findSharedItems(t, searchPath)
	if foundSecret {
		t.Fatalf("secret not deleted")
	}
	// clear out dv for next run
	d.deleteVolume(t.Name())

}

func TestDeleteSecretVolume(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	_, searchPath := primeSecretVolume(t, d, targetPath, nil, k8sClient, shareClient)
	err = d.deleteVolume(t.Name())
	if err != nil {
		t.Fatalf("unexpected error on delete volume: %s", err.Error())
	}
	foundSecret, _ := findSharedItems(t, searchPath)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
}

func TestChangeKeys(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	secret, searchPath := primeSecretVolume(t, d, targetPath, nil, k8sClient, shareClient)
	foundSecret, _ := findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("did not find secret in mount path")
	}

	delete(secret.Data, secretkey1)
	secretkey3 := "secretkey3"
	secretvalue3 := "secretvalue3"
	secret.Data[secretkey3] = []byte(secretvalue3)
	cache.UpsertSecret(secret)
	foundSecret, _ = findSharedItems(t, searchPath)
	if foundSecret {
		t.Fatalf("found old key secretkey1")
	}
	filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && strings.Contains(info.Name(), secretkey3) {
			foundSecret = true
		}
		return nil
	})
	if !foundSecret {
		t.Fatalf("did not find key secretkey3")
	}
	// delete dv for next run
	d.deleteVolume(t.Name())

}

func TestDeleteConfigMapVolume(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	_, searchPath := primeConfigMapVolume(t, d, targetPath, nil, k8sClient, shareClient)
	err = d.deleteVolume(t.Name())
	if err != nil {
		t.Fatalf("unexpected error on delete volume: %s", err.Error())
	}
	_, foundConfigMap := findSharedItems(t, searchPath)

	if foundConfigMap {
		t.Fatalf("configmap not deleted ro")
	}

}

func TestDeleteShare(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)

	k8sClient := fakekubeclientset.NewSimpleClientset()
	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	client.SetClient(k8sClient)

	share := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TestDeleteShare",
		},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{
				Name:      "secret1",
				Namespace: "namespace",
			},

			Description: "",
		},
		Status: sharev1alpha1.SharedSecretStatus{},
	}

	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	_, searchPath := primeSecretVolume(t, d, targetPath, share, k8sClient, shareClient)
	foundSecret, _ := findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cache.DelSharedSecret(share)
	foundSecret, _ = findSharedItems(t, searchPath)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
	// clear our dv for next run
	d.deleteVolume(t.Name())

}

func TestDeleteReAddShare(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)

	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)

	share := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "TestDeleteReAddShare",
			ResourceVersion: "1",
		},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{
				Name:      "secret1",
				Namespace: "namespace",
			},
			Description: "",
		},
		Status: sharev1alpha1.SharedSecretStatus{},
	}

	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	_, searchPath := primeSecretVolume(t, d, targetPath, share, k8sClient, shareClient)
	foundSecret, _ := findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cache.DelSharedSecret(share)
	foundSecret, _ = findSharedItems(t, searchPath)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}

	share.ResourceVersion = "2"
	cache.AddSharedSecret(share)
	foundSecret, _ = findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("secret not found after readd")
	}
	// clear out dv for next run
	d.deleteVolume(t.Name())

}

func TestPruner(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)

	dv := &driverVolume{VolID: "vol", PodNamespace: "ns", PodName: "pod", Lock: &sync.Mutex{}}
	dv.StoreToDisk(d.GetVolMapRoot())
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)

	d.Prune(k8sClient)
	prunedFile := filepath.Join(d.GetVolMapRoot(), "vol")
	filepath.Walk(d.GetVolMapRoot(), func(path string, info fs.FileInfo, err error) error {
		if path == prunedFile {
			t.Fatalf("file %q was not pruned", path)
		}
		return nil
	})
}

func TestUpdateShare(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient := fakekubeclientset.NewSimpleClientset()
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	client.SetClient(k8sClient)

	share := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "TestUpdateShare",
			ResourceVersion: "1",
		},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{
				Name:      "secret1",
				Namespace: "namespace",
			},
			Description: "",
		},
		Status: sharev1alpha1.SharedSecretStatus{},
	}

	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	_, searchPath := primeSecretVolume(t, d, targetPath, share, k8sClient, shareClient)
	foundSecret, _ := findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	// change share to a different secret (no longer support switch between secret and crd with now having different CRDs for each)
	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret2",
			Namespace: "namespace",
		},
		Data: map[string][]byte{secretkey2: []byte(secretvalue2)},
	}
	cache.UpsertSecret(secret2)
	share.Spec.SecretRef.Name = "secret2"
	share.ResourceVersion = "2"
	cache.UpdateSharedSecret(share)

	foundSecret, _ = findSharedItems(t, searchPath)

	if !foundSecret {
		t.Fatalf("secret still should have been found")
	}

	// clear out dv for next run
	d.deleteVolume(t.Name())

}

func TestPermChanges(t *testing.T) {
	d, dir1, dir2, err := testDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), t.Name())
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)

	share := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TestPermChanges",
		},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{
				Name:      "secret1",
				Namespace: "namespace",
			},
			Description: "",
		},
		Status: sharev1alpha1.SharedSecretStatus{},
	}

	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	_, searchPath := primeSecretVolume(t, d, targetPath, share, k8sClient, shareClient)
	foundSecret, _ := findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	denyReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: false}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", denyReactorFunc)

	shareUpdateRanger(share.Name, share)

	foundSecret, _ = findSharedItems(t, searchPath)
	if foundSecret {
		t.Fatalf("secret should have been removed")
	}

	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)

	shareUpdateRanger(share.Name, share)

	foundSecret, _ = findSharedItems(t, searchPath)
	if !foundSecret {
		t.Fatalf("secret should have been found")
	}
	// clear out dv for next run
	d.deleteVolume(t.Name())

}

// TestMapVolumeToPodWithKubeClient creates a new CSIDriver with a kubernetes client, which
// changes the behavior of the component, so instead of directly reading backing-resources from the
// object-cache, it directly updates the cache before trying to mount the volume.
func TestMapVolumeToPodWithKubeClient(t *testing.T) {
	k8sClient := fakekubeclientset.NewSimpleClientset()
	client.SetClient(k8sClient)
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "secret-namespace",
		},
		Data: map[string][]byte{secretkey1: []byte(secretvalue1)},
	}
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap",
			Namespace: "configmap-namespace",
		},
		Data: map[string]string{configmapkey1: configmapvalue1},
	}
	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	secretReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &secret, nil
	}
	k8sClient.PrependReactor("get", "secrets", secretReactorFunc)
	configMapReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &configMap, nil
	}
	k8sClient.PrependReactor("get", "configmaps", configMapReactorFunc)
	shareClient := fakeshareclientset.NewSimpleClientset()
	client.SetShareClient(shareClient)
	cmShare := &sharev1alpha1.SharedConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", t.Name()),
			Namespace: metav1.NamespaceDefault,
		},
		Spec: sharev1alpha1.SharedConfigMapSpec{
			ConfigMapRef: sharev1alpha1.SharedConfigMapReference{
				Name:      configMap.GetName(),
				Namespace: configMap.GetNamespace(),
			},
			Description: "",
		},
		Status: sharev1alpha1.SharedConfigMapStatus{},
	}
	sharedConfigMapReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, cmShare, nil
	}
	shareClient.PrependReactor("get", "sharedconfigmaps", sharedConfigMapReactorFunc)
	sShare := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", t.Name()),
			Namespace: metav1.NamespaceDefault,
		},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{
				Name:      secret.GetName(),
				Namespace: secret.GetNamespace(),
			},
			Description: "",
		},
		Status: sharev1alpha1.SharedSecretStatus{},
	}
	sharedSecretConfigMapReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, sShare, nil
	}
	shareClient.PrependReactor("get", "sharedsecrets", sharedSecretConfigMapReactorFunc)

	shareInformerFactory := shareinformer.NewSharedInformerFactoryWithOptions(shareClient,
		10*time.Minute)
	client.SetSharedSecretsLister(shareInformerFactory.Sharedresource().V1alpha1().SharedSecrets().Lister())
	client.SetSharedConfigMapsLister(shareInformerFactory.Sharedresource().V1alpha1().SharedConfigMaps().Lister())

	tests := []struct {
		name    string
		sShare  *sharev1alpha1.SharedSecret
		cmShare *sharev1alpha1.SharedConfigMap
	}{{
		name:   "Secret",
		sShare: sShare,
	}, {
		name:    "ConfigMap",
		cmShare: cmShare,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.cmShare != nil {
				cache.AddSharedConfigMap(test.cmShare)
			}
			if test.sShare != nil {
				cache.AddSharedSecret(test.sShare)
			}

			// making sure the tests are running on temporary directories, those will be deleted at
			// the end of each test pass
			targetPath, err := ioutil.TempDir(os.TempDir(), test.name)
			if err != nil {
				t.Fatalf("err on targetPath %s", err.Error())
			}
			d, dir1, dir2, err := testDriver(test.name, k8sClient)
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir1)
			defer os.RemoveAll(dir2)

			// creating driverVolume only for this test
			volCtx := seedVolumeContext()
			dv, err := d.createVolume(test.name, targetPath, true, volCtx, test.cmShare, test.sShare, 0, mountAccess)
			if err != nil {
				t.Fatalf("unexpected error on createVolume: '%s'", err.Error())
			}

			// creating the mount point infrastructure, materializing the objects in cache as files
			if err = d.mapVolumeToPod(dv); err != nil {
				t.Fatalf("unexpected error on mapVolumeToPod: '%s'", err.Error())
			}

			// inspecting bind directory looking for files originated from testing resources
			foundSecret, foundConfigMap := findSharedItems(t, targetPath)
			t.Logf("mount point contents: secret='%v', configmap='%v'", foundSecret, foundConfigMap)
			if !foundSecret && !foundConfigMap {
				t.Fatalf("mount point doesn't have data: secret='%v', configmap='%v'", foundSecret, foundConfigMap)
			}
		})
	}
}

func primeSecretVolume(t *testing.T, d CSIDriver, targetPath string, share *sharev1alpha1.SharedSecret, k8sClient *fakekubeclientset.Clientset, shareClient *fakeshareclientset.Clientset) (*corev1.Secret, string) {
	volCtx := seedVolumeContext()
	if share == nil {
		share = &sharev1alpha1.SharedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.Name(),
			},
			Spec: sharev1alpha1.SharedSecretSpec{
				SecretRef: sharev1alpha1.SharedSecretReference{
					Name:      "secret1",
					Namespace: "namespace",
				},

				Description: "",
			},
			Status: sharev1alpha1.SharedSecretStatus{},
		}
	}
	cache.AddSharedSecret(share)

	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	shareSecretReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, share, nil
	}
	shareSecretListReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		list := sharev1alpha1.SharedSecretList{Items: []sharev1alpha1.SharedSecret{*share}}
		return true, &list, nil
	}
	shareClient.PrependReactor("get", "sharedsecrets", shareSecretReactorFunc)
	shareClient.PrependReactor("list", "sharedsecrets", shareSecretListReactorFunc)
	shareInformerFactory := shareinformer.NewSharedInformerFactoryWithOptions(shareClient, 10*time.Minute)
	client.SetSharedSecretsLister(shareInformerFactory.Sharedresource().V1alpha1().SharedSecrets().Lister())

	dv, err := d.createVolume(t.Name(), targetPath, true, volCtx, nil, share, 0, mountAccess)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "namespace",
		},
		Data: map[string][]byte{secretkey1: []byte(secretvalue1)},
	}

	cache.UpdateSharedSecret(share)

	secretReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, secret, nil
	}
	k8sClient.PrependReactor("get", "secrets", secretReactorFunc)
	cache.UpsertSecret(secret)
	err = d.mapVolumeToPod(dv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return secret, targetPath
}

func primeConfigMapVolume(t *testing.T, d CSIDriver, targetPath string, share *sharev1alpha1.SharedConfigMap, k8sClient *fakekubeclientset.Clientset, shareClient *fakeshareclientset.Clientset) (*corev1.ConfigMap, string) {
	volCtx := seedVolumeContext()
	if share == nil {
		share = &sharev1alpha1.SharedConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.Name(),
			},
			Spec: sharev1alpha1.SharedConfigMapSpec{
				ConfigMapRef: sharev1alpha1.SharedConfigMapReference{
					Name:      "configmap1",
					Namespace: "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.SharedConfigMapStatus{},
		}
	}
	cache.AddSharedConfigMap(share)

	acceptReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	}
	k8sClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	shareConfigMapReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, share, nil
	}
	shareConfigMapListReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		list := sharev1alpha1.SharedConfigMapList{Items: []sharev1alpha1.SharedConfigMap{*share}}
		return true, &list, nil
	}
	shareClient.PrependReactor("get", "sharedconfigmaps", shareConfigMapReactorFunc)
	shareClient.PrependReactor("list", "sharedconfigmaps", shareConfigMapListReactorFunc)
	shareInformerFactory := shareinformer.NewSharedInformerFactoryWithOptions(shareClient, 10*time.Minute)
	client.SetSharedConfigMapsLister(shareInformerFactory.Sharedresource().V1alpha1().SharedConfigMaps().Lister())

	dv, err := d.createVolume(t.Name(), targetPath, true, volCtx, share, nil, 0, mountAccess)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap1",
			Namespace: "namespace",
		},
		Data: map[string]string{configmapkey1: configmapvalue1},
	}

	cache.UpdateSharedConfigMap(share)

	configMapReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, cm, nil
	}
	k8sClient.PrependReactor("get", "configmaps", configMapReactorFunc)
	cache.UpsertConfigMap(cm)
	err = d.mapVolumeToPod(dv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return cm, targetPath
}

func findSharedItems(t *testing.T, dir string) (bool, bool) {
	foundSecret := false
	foundConfigMap := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		t.Logf("found file %s dir flag %v", info.Name(), info.IsDir())
		if err == nil && strings.Contains(info.Name(), secretkey1) {
			foundSecret = true
		}
		if err == nil && strings.Contains(info.Name(), secretkey2) {
			foundSecret = true
		}
		if err == nil && strings.Contains(info.Name(), configmapkey1) {
			foundConfigMap = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected walk error: %s", err.Error())
	}
	return foundSecret, foundConfigMap
}
