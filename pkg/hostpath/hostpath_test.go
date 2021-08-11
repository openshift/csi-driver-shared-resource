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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubetesting "k8s.io/client-go/testing"

	sharev1alpha1 "github.com/openshift/csi-driver-shared-resource/pkg/api/sharedresource/v1alpha1"
	"github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

const (
	secretkey1      = "secretkey1"
	secretvalue1    = "secretvalue1"
	configmapkey1   = "configmapkey1"
	configmapvalue1 = "configmapvalue1"
)

func testHostPathDriver(testName string) (*hostPath, string, string, error) {
	tmpDir1, err := ioutil.TempDir(os.TempDir(), testName)
	if err != nil {
		return nil, "", "", err
	}
	tmpDir2, err := ioutil.TempDir(os.TempDir(), testName)
	if err != nil {
		return nil, "", "", err
	}
	hp, err := NewHostPathDriver(tmpDir1, tmpDir2, testName, "nodeID1", "endpoint1", 0, "version1")
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
	hp, dir1, dir2, err := testHostPathDriver(t.Name())
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	volCtx := seedVolumeContext()
	_, err = hp.createHostpathVolume(t.Name(), "", false, volCtx, &sharev1alpha1.Share{}, 0, mountAccess+1)
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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
			t.Fatalf("unexpeted error on delete volume: %s", err.Error())
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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
		secretkey2 := "secretkey2"
		secretvalue2 := "secretvalue2"
		secret.Data[secretkey2] = []byte(secretvalue2)
		cache.UpsertSecret(secret)
		foundSecret, _ = findSharedItems(t, searchPath)
		if foundSecret {
			t.Fatalf("found old key secretkey1")
		}
		filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && strings.Contains(info.Name(), secretkey2) {
				foundSecret = true
			}
			return nil
		})
		if !foundSecret {
			t.Fatalf("did not find key secretkey2")
		}
		// delete hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestDeleteConfigMapVolume(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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
			t.Fatalf("unexpeted error on delete volume: %s", err.Error())
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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

		share := &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: "TestDeleteShare",
			},
			Spec: sharev1alpha1.ShareSpec{
				BackingResource: sharev1alpha1.BackingResource{
					Kind:       "Secret",
					APIVersion: "v1",
					Name:       "secret1",
					Namespace:  "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.ShareStatus{},
		}
		shareLister := &fakeShareLister{
			share: share,
		}
		client.SetSharesLister(shareLister)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
		foundSecret, _ := findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("secret not found")
		}

		cache.DelShare(share)
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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

		share := &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "TestDeleteReAddShare",
				ResourceVersion: "1",
			},
			Spec: sharev1alpha1.ShareSpec{
				BackingResource: sharev1alpha1.BackingResource{
					Kind:       "Secret",
					APIVersion: "v1",
					Name:       "secret1",
					Namespace:  "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.ShareStatus{},
		}
		shareLister := &fakeShareLister{
			share: share,
		}
		client.SetSharesLister(shareLister)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
		foundSecret, _ := findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("secret not found")
		}

		cache.DelShare(share)
		foundSecret, _ = findSharedItems(t, searchPath)

		if foundSecret {
			t.Fatalf("secret not deleted")
		}

		share.ResourceVersion = "2"
		cache.AddShare(share)
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
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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

		share := &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "TestUpdateShare",
				ResourceVersion: "1",
			},
			Spec: sharev1alpha1.ShareSpec{
				BackingResource: sharev1alpha1.BackingResource{
					Kind:       "Secret",
					APIVersion: "v1",
					Name:       "secret1",
					Namespace:  "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.ShareStatus{},
		}
		shareLister := &fakeShareLister{
			share: share,
		}
		client.SetSharesLister(shareLister)
		cache.AddShare(share)

		_, searchPath := primeSecretVolume(t, hp, targetPath, ro, share)
		foundSecret, _ := findSharedItems(t, searchPath)
		if !foundSecret {
			t.Fatalf("secret not found")
		}

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: "namespace",
			},
			Data: map[string]string{configmapkey1: configmapvalue1},
		}
		cache.UpsertConfigMap(cm)

		share.Spec.BackingResource.Kind = "ConfigMap"
		share.Spec.BackingResource.Name = "configmap1"

		share.ResourceVersion = "2"
		cache.UpdateShare(share)
		foundSecret, foundConfigMap := findSharedItems(t, searchPath)
		if foundSecret {
			t.Fatalf("secret should have been removed")
		}
		if !foundConfigMap {
			t.Fatalf("configmap should have been found")
		}
		// clear out hpv for next run
		hp.deleteHostpathVolume(t.Name())
	}
}

func TestPermChanges(t *testing.T) {
	readOnly := []bool{true, false}
	for _, ro := range readOnly {
		hp, dir1, dir2, err := testHostPathDriver(t.Name())
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

		share := &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: "TestPermChanges",
			},
			Spec: sharev1alpha1.ShareSpec{
				BackingResource: sharev1alpha1.BackingResource{
					Kind:       "Secret",
					APIVersion: "v1",
					Name:       "secret1",
					Namespace:  "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.ShareStatus{},
		}
		shareLister := &fakeShareLister{
			share: share,
		}
		client.SetSharesLister(shareLister)
		cache.AddShare(share)

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

func primeSecretVolume(t *testing.T, hp *hostPath, targetPath string, readOnly bool, share *sharev1alpha1.Share) (*corev1.Secret, string) {
	volCtx := seedVolumeContext()
	if share == nil {
		share = &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.Name(),
			},
			Spec: sharev1alpha1.ShareSpec{
				BackingResource: sharev1alpha1.BackingResource{
					Kind:       "Secret",
					APIVersion: "v1",
					Name:       "secret1",
					Namespace:  "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.ShareStatus{},
		}
		shareLister := &fakeShareLister{
			share: share,
		}
		client.SetSharesLister(shareLister)
		cache.AddShare(share)
	}
	hpv, err := hp.createHostpathVolume(t.Name(), targetPath, readOnly, volCtx, share, 0, mountAccess)
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

	cache.UpdateShare(share)

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

func primeConfigMapVolume(t *testing.T, hp *hostPath, targetPath string, readOnly bool, share *sharev1alpha1.Share) (*corev1.ConfigMap, string) {
	volCtx := seedVolumeContext()
	if share == nil {
		share = &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.Name(),
			},
			Spec: sharev1alpha1.ShareSpec{
				BackingResource: sharev1alpha1.BackingResource{
					Kind:       "ConfigMap",
					APIVersion: "v1",
					Name:       "configmap1",
					Namespace:  "namespace",
				},
				Description: "",
			},
			Status: sharev1alpha1.ShareStatus{},
		}
		shareLister := &fakeShareLister{
			share: share,
		}
		client.SetSharesLister(shareLister)
		cache.AddShare(share)
	}
	hpv, err := hp.createHostpathVolume(t.Name(), targetPath, readOnly, volCtx, share, 0, mountAccess)
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

	cache.UpdateShare(share)

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
