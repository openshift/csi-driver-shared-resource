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
	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
	"io/ioutil"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubetesting "k8s.io/client-go/testing"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/csi-driver-projected-resource/pkg/cache"
	"github.com/openshift/csi-driver-projected-resource/pkg/client"
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
	_, err = hp.createHostpathVolume(t.Name(), "", volCtx, &sharev1alpha1.Share{}, 0, mountAccess+1)
	if err == nil {
		t.Fatalf("err nil unexpectedly")
	}
	if !strings.Contains(err.Error(), "unsupported access type") {
		t.Fatalf("unexpected err %s", err.Error())
	}
}

func TestCreateDeleteConfigMap(t *testing.T) {
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
	cm := primeConfigMapVolume(t, hp, targetPath, nil)
	_, foundConfigMap := findSharedItems(t, targetPath)
	if !foundConfigMap {
		t.Fatalf("did not find configmap in mount path")
	}
	cache.DelConfigMap(cm)
	_, foundConfigMap = findSharedItems(t, targetPath)
	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
}

func TestCreateDeleteSecret(t *testing.T) {
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
	secret := primeSecretVolume(t, hp, targetPath, nil)
	foundSecret, _ := findSharedItems(t, targetPath)
	if !foundSecret {
		t.Fatalf("did not find secret in mount path")
	}
	cache.DelSecret(secret)
	foundSecret, _ = findSharedItems(t, targetPath)
	if foundSecret {
		t.Fatalf("secret not deleted")
	}
}

func TestDeleteSecretVolume(t *testing.T) {
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
	primeSecretVolume(t, hp, targetPath, nil)
	err = hp.deleteHostpathVolume(t.Name())
	if err != nil {
		t.Fatalf("unexpeted error on delete volume: %s", err.Error())
	}
	foundSecret, _ := findSharedItems(t, dir1)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
	if empty, err := isDirEmpty(dir1); !empty || err != nil {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		t.Fatalf("volume directory not cleaned out empty %v err %s", empty, errStr)
	}

}

func TestChangeKeys(t *testing.T) {
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
	secret := primeSecretVolume(t, hp, targetPath, nil)
	foundSecret, _ := findSharedItems(t, targetPath)
	if !foundSecret {
		t.Fatalf("did not find secret in mount path")
	}

	delete(secret.Data, secretkey1)
	secretkey2 := "secretkey2"
	secretvalue2 := "secretvalue2"
	secret.Data[secretkey2] = []byte(secretvalue2)
	cache.UpsertSecret(secret)
	foundSecret, _ = findSharedItems(t, targetPath)
	if foundSecret {
		t.Fatalf("found old key secretkey1")
	}
	filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && strings.Contains(info.Name(), secretkey2) {
			foundSecret = true
		}
		return nil
	})
	if !foundSecret {
		t.Fatalf("did not find key secretkey2")
	}
}

func TestDeleteConfigMapVolume(t *testing.T) {
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
	primeConfigMapVolume(t, hp, targetPath, nil)
	err = hp.deleteHostpathVolume(t.Name())
	if err != nil {
		t.Fatalf("unexpeted error on delete volume: %s", err.Error())
	}
	_, foundConfigMap := findSharedItems(t, dir1)

	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
	if empty, err := isDirEmpty(dir1); !empty || err != nil {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		t.Fatalf("volume directory not cleaned out empty %v err %s", empty, errStr)
	}
}

func TestDeleteShare(t *testing.T) {
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

	primeSecretVolume(t, hp, targetPath, share)
	foundSecret, _ := findSharedItems(t, targetPath)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cache.DelShare(share)
	foundSecret, _ = findSharedItems(t, targetPath)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
}

func TestDeleteReAddShare(t *testing.T) {
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

	primeSecretVolume(t, hp, targetPath, share)
	foundSecret, _ := findSharedItems(t, targetPath)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cache.DelShare(share)
	foundSecret, _ = findSharedItems(t, targetPath)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}

	share.ResourceVersion = "2"
	cache.AddShare(share)
	foundSecret, _ = findSharedItems(t, targetPath)
	if !foundSecret {
		t.Fatalf("secret not found after readd")
	}
}

func TestUpdateShare(t *testing.T) {
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

	primeSecretVolume(t, hp, targetPath, share)
	foundSecret, _ := findSharedItems(t, targetPath)
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
	foundSecret, foundConfigMap := findSharedItems(t, targetPath)
	if foundSecret {
		t.Fatalf("secret should have been removed")
	}
	if !foundConfigMap {
		t.Fatalf("configmap should have been found")
	}
}

func TestPermChanges(t *testing.T) {
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

	primeSecretVolume(t, hp, targetPath, share)
	foundSecret, _ := findSharedItems(t, targetPath)
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

	foundSecret, _ = findSharedItems(t, targetPath)
	if foundSecret {
		t.Fatalf("secret should have been removed")
	}

	sarClient = fakekubeclientset.NewSimpleClientset()
	sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	client.SetClient(sarClient)

	shareUpdateRanger(share.Name, share)

	foundSecret, _ = findSharedItems(t, targetPath)
	if !foundSecret {
		t.Fatalf("secret should have been found")
	}
}

func primeSecretVolume(t *testing.T, hp *hostPath, targetPath string, share *sharev1alpha1.Share) *corev1.Secret {
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
	hpv, err := hp.createHostpathVolume(t.Name(), targetPath, volCtx, share, 0, mountAccess)
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
	return secret
}

func primeConfigMapVolume(t *testing.T, hp *hostPath, targetPath string, share *sharev1alpha1.Share) *corev1.ConfigMap {
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
	hpv, err := hp.createHostpathVolume(t.Name(), targetPath, volCtx, share, 0, mountAccess)
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
	return cm
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
