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

package hostpath

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubetesting "k8s.io/client-go/testing"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
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

func testHostPathDriver(testName string, kubeClient kubernetes.Interface) (*hostPath, string, string, error) {
	tmpDir1, err := ioutil.TempDir(os.TempDir(), testName)
	if err != nil {
		return nil, "", "", err
	}
	tmpDir2, err := ioutil.TempDir(os.TempDir(), testName)
	if err != nil {
		return nil, "", "", err
	}
	hp, err := NewHostPathDriver(tmpDir1, tmpDir2, testName, "nodeID1", "endpoint1", 0, "version1", kubeClient)
	return hp, tmpDir1, tmpDir2, err
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

func TestCreateHostPathVolumeBadAccessType(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	volCtx := seedVolumeContext()
	_, err = hp.createHostpathVolume(t.Name(), "", false, volCtx, &sharev1alpha1.SharedConfigMap{}, nil, 0, mountAccess+1)
	if err == nil {
		t.Fatalf("err nil unexpectedly")
	}
	if !strings.Contains(err.Error(), "unsupported access type") {
		t.Fatalf("unexpected err %s", err.Error())
	}
}

func TestCreateDeleteConfigMap(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		cm, searchPath := primeConfigMapVolume(t, hp, targetPath, ro, nil)
		_, foundConfigMap := findSharedItems(t, searchPath)
		if !foundConfigMap {
			t.Fatalf("did not find configmap in mount path ro %v", ro)
		}
		cache.DelConfigMap(cm)
		_, foundConfigMap = findSharedItems(t, searchPath)
		if foundConfigMap {
			t.Fatalf("configmap not deleted ro %v", ro)
		}
		// clear out hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestCreateDeleteSecret(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		secret, searchPath := primeSecretVolume(t, hp, targetPath, ro, nil)
		foundSecret, _ := findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("did not find secret in mount path")
		}
		cache.DelSecret(secret)
		foundSecret, _ = findSharedItems(t, searchPath)
		if foundSecret {
			t.Fatalf("secret not deleted")
		}
		// clear out hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestDeleteSecretVolume(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, nil)
		err = hp.deleteHostpathVolume(t.Name())
		if err != nil {
			t.Fatalf("unexpected error on delete volume: %s", err.Error())
		}
		foundSecret, _ := findSharedItems(t, searchPath)

		if foundSecret {
			t.Fatalf("secret not deleted")
		}
		if ro {
			if empty, err := isDirEmpty(searchPath); !empty || err != nil {
				errStr := ""
				if err != nil {
					errStr = err.Error()
				}
				t.Fatalf("volume directory not cleaned out empty %v err %s", empty, errStr)
			}
		}

	}

}

func TestChangeKeys(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		secret, searchPath := primeSecretVolume(t, hp, targetPath, ro, nil)
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
		// delete hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestDeleteConfigMapVolume(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		_, searchPath := primeConfigMapVolume(t, hp, targetPath, ro, nil)
		err = hp.deleteHostpathVolume(t.Name())
		if err != nil {
			t.Fatalf("unexpected error on delete volume: %s", err.Error())
		}
		_, foundConfigMap := findSharedItems(t, searchPath)

		if foundConfigMap {
			t.Fatalf("configmap not deleted ro %v", ro)
		}
		if ro {
			if empty, err := isDirEmpty(searchPath); !empty || err != nil {
				errStr := ""
				if err != nil {
					errStr = err.Error()
				}
				t.Fatalf("volume directory not cleaned out empty %v err %s", empty, errStr)
			}
		}

	}
}

func TestDeleteShare(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		sarClient := fakekubeclientset.NewSimpleClientset()
		sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
		client.SetClient(sarClient)

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
		shareLister := &fakeSharedSecretLister{
			sShare: share,
		}
		client.SetSharedSecretsLister(shareLister)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
		foundSecret, _ := findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("secret not found")
		}

		cache.DelSharedSecret(share)
		foundSecret, _ = findSharedItems(t, searchPath)

		if foundSecret {
			t.Fatalf("secret not deleted")
		}
		// clear our hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestDeleteReAddShare(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		sarClient := fakekubeclientset.NewSimpleClientset()
		sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
		client.SetClient(sarClient)

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
		shareLister := &fakeSharedSecretLister{
			sShare: share,
		}
		client.SetSharedSecretsLister(shareLister)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
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
		// clear out hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestUpdateShare(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		sarClient := fakekubeclientset.NewSimpleClientset()
		sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
		client.SetClient(sarClient)

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
		shareLister := &fakeSharedSecretLister{
			sShare: share,
		}
		client.SetSharedSecretsLister(shareLister)
		cache.AddSharedSecret(share)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
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

		// clear out hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestPermChanges(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name(), nil)
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
		sarClient := fakekubeclientset.NewSimpleClientset()
		sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
		client.SetClient(sarClient)

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
		shareLister := &fakeSharedSecretLister{
			sShare: share,
		}
		client.SetSharedSecretsLister(shareLister)
		cache.AddSharedSecret(share)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
		foundSecret, _ := findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("secret not found")
		}

		denyReactorFunc := func(action fakekubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: false}}, nil
		}
		sarClient = fakekubeclientset.NewSimpleClientset()
		sarClient.PrependReactor("create", "subjectaccessreviews", denyReactorFunc)
		client.SetClient(sarClient)

		shareUpdateRanger(share.Name, share)

		foundSecret, _ = findSharedItems(t, searchPath)
		if foundSecret {
			t.Fatalf("secret should have been removed")
		}

		sarClient = fakekubeclientset.NewSimpleClientset()
		sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
		client.SetClient(sarClient)

		shareUpdateRanger(share.Name, share)

		foundSecret, _ = findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("secret should have been found")
		}
		// clear out hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

// TestMapVolumeToPodWithKubeClient creates a new HostPathDriver with a kubernetes client, which
// changes the behavior of the component, so instead of directly reading backing-resources from the
// object-cache, it directly updates the cache before trying to mount the volume.
func TestMapVolumeToPodWithKubeClient(t *testing.T) {
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

	tests := []struct {
		name       string
		sShare     *sharev1alpha1.SharedSecret
		cmShare    *sharev1alpha1.SharedConfigMap
		kubeClient kubernetes.Interface
	}{{
		name: "Secret",
		sShare: &sharev1alpha1.SharedSecret{
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
		},
		kubeClient: fakekubeclientset.NewSimpleClientset(&secret),
	}, {
		name: "ConfigMap",
		cmShare: &sharev1alpha1.SharedConfigMap{
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
		},
		kubeClient: fakekubeclientset.NewSimpleClientset(&configMap),
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
			hp, dir1, dir2, err := testHostPathDriver(test.name, test.kubeClient)
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir1)
			defer os.RemoveAll(dir2)

			// creating hostPathVolume only for this test
			volCtx := seedVolumeContext()
			hpv, err := hp.createHostpathVolume(test.name, targetPath, true, volCtx, test.cmShare, test.sShare, 0, mountAccess)
			if err != nil {
				t.Fatalf("unexpected error on createHostpathVolume: '%s'", err.Error())
			}

			// creating the mount point infrastructure, materializing the objects in cache as files
			if err = hp.mapVolumeToPod(hpv); err != nil {
				t.Fatalf("unexpected error on mapVolumeToPod: '%s'", err.Error())
			}

			// given it's a read-only mount point, the target-path needs to be amended with the inner
			// bind directory
			bindDir := filepath.Join(hp.root, "bind-dir")

			// inspecting bind directory looking for files originated from testing resources
			foundSecret, foundConfigMap := findSharedItems(t, bindDir)
			t.Logf("mount point contents: secret='%v', configmap='%v'", foundSecret, foundConfigMap)
			if !foundSecret && !foundConfigMap {
				t.Fatalf("mount point doesn't have data: secret='%v', configmap='%v'", foundSecret, foundConfigMap)
			}
		})
	}
}

func primeSecretVolume(t *testing.T, hp *hostPath, targetPath string, readOnly bool, share *sharev1alpha1.SharedSecret) (*corev1.Secret, string) {
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
		shareLister := &fakeSharedSecretLister{
			sShare: share,
		}
		client.SetSharedSecretsLister(shareLister)
		cache.AddSharedSecret(share)
	}
	hpv, err := hp.createHostpathVolume(t.Name(), targetPath, readOnly, volCtx, nil, share, 0, mountAccess)
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

	cache.UpsertSecret(secret)
	err = hp.mapVolumeToPod(hpv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	if readOnly {
		return secret, filepath.Join(hp.root, "bind-dir")
	}
	return secret, targetPath
}

func primeConfigMapVolume(t *testing.T, hp *hostPath, targetPath string, readOnly bool, share *sharev1alpha1.SharedConfigMap) (*corev1.ConfigMap, string) {
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
		shareLister := &fakeSharedConfigMapLister{
			cmShare: share,
		}
		client.SetSharedConfigMapsLister(shareLister)
		cache.AddSharedConfigMap(share)
	}
	hpv, err := hp.createHostpathVolume(t.Name(), targetPath, readOnly, volCtx, share, nil, 0, mountAccess)
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

	cache.UpsertConfigMap(cm)
	err = hp.mapVolumeToPod(hpv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	if readOnly {
		return cm, filepath.Join(hp.root, "bind-dir")
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
