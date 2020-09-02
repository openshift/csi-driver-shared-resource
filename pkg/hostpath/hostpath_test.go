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

func testHostPathDriver() (*hostPath, string, string, error) {
	tmpDir1, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		return nil, "", "", err
	}
	tmpDir2, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		return nil, "", "", err
	}
	hp, err := NewHostPathDriver(tmpDir1, tmpDir2, "ut-driver", "nodeID1", "endpoint1", 0, "version1")
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
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	volCtx := seedVolumeContext()
	_, err = hp.createHostpathVolume("volID", "", volCtx, &sharev1alpha1.Share{}, 0, mountAccess+1)
	if err == nil {
		t.Fatalf("err nil unexpectedly")
	}
	if !strings.Contains(err.Error(), "unsupported access type") {
		t.Fatalf("unexpected err %s", err.Error())
	}
}

func TestCreateConfigMapHostPathVolume(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	cm := primeConfigMapVolume(hp, targetPath, nil, t)
	_, foundConfigMap := findSharedItems(targetPath, t)
	if !foundConfigMap {
		t.Fatalf("did not find configmap in mount path")
	}
	cache.DelConfigMap(cm)
	_, foundConfigMap = findSharedItems(targetPath, t)
	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
}

func TestCreateSecretHostPathVolume(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	secret := primeSecretVolume(hp, targetPath, nil, t)
	foundSecret, _ := findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("did not find secret in mount path")
	}
	cache.DelSecret(secret)
	foundSecret, _ = findSharedItems(targetPath, t)
	if foundSecret {
		t.Fatalf("secret not deleted")
	}
}

func TestDeleteSecretVolume(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	primeSecretVolume(hp, targetPath, nil, t)
	err = hp.deleteHostpathVolume("volID")
	if err != nil {
		t.Fatalf("unexpeted error on delete volume: %s", err.Error())
	}
	foundSecret, _ := findSharedItems(dir1, t)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
	if empty, err := isDirEmpty(dir1); !empty || err != nil {
		t.Fatalf("volume directory not cleaned out empty %v err %s", empty, err.Error())
	}

}

func TestDeleteConfigMapVolume(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	primeConfigMapVolume(hp, targetPath, nil, t)
	err = hp.deleteHostpathVolume("volID")
	if err != nil {
		t.Fatalf("unexpeted error on delete volume: %s", err.Error())
	}
	_, foundConfigMap := findSharedItems(dir1, t)

	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
	if empty, err := isDirEmpty(dir1); !empty || err != nil {
		t.Fatalf("volume directory not cleaned out empty %v err %s", empty, err.Error())
	}

}

func TestDeleteShare(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
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
			Name: "share1",
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

	primeSecretVolume(hp, targetPath, share, t)
	foundSecret, _ := findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cache.DelShare(share)
	foundSecret, _ = findSharedItems(targetPath, t)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
}

func TestDeleteReAddShare(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
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
			Name: "share1",
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

	primeSecretVolume(hp, targetPath, share, t)
	foundSecret, _ := findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cache.DelShare(share)
	foundSecret, _ = findSharedItems(targetPath, t)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}

	cache.AddShare(share)
	foundSecret, _ = findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("secret not found after readd")
	}
}

func TestUpdateShare(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
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
			Name: "share1",
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

	primeSecretVolume(hp, targetPath, share, t)
	foundSecret, _ := findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("secret not found")
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap1",
			Namespace: "namespace",
		},
	}
	cache.UpsertConfigMap(cm)

	share.Spec.BackingResource.Kind = "ConfigMap"
	share.Spec.BackingResource.Name = "configmap1"

	cache.UpdateShare(share)
	foundSecret, foundConfigMap := findSharedItems(targetPath, t)
	if foundSecret {
		t.Fatalf("secret should have been removed")
	}
	if !foundConfigMap {
		t.Fatalf("configmap should have been found")
	}
}

func TestPermChanges(t *testing.T) {
	hp, dir1, dir2, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
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
			Name: "share1",
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

	primeSecretVolume(hp, targetPath, share, t)
	foundSecret, _ := findSharedItems(targetPath, t)
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

	foundSecret, _ = findSharedItems(targetPath, t)
	if foundSecret {
		t.Fatalf("secret should have been removed")
	}

	sarClient = fakekubeclientset.NewSimpleClientset()
	sarClient.PrependReactor("create", "subjectaccessreviews", acceptReactorFunc)
	client.SetClient(sarClient)

	shareUpdateRanger(share.Name, share)

	foundSecret, _ = findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("secret should have been found")
	}
}

func primeSecretVolume(hp *hostPath, targetPath string, share *sharev1alpha1.Share, t *testing.T) *corev1.Secret {
	volCtx := seedVolumeContext()
	if share == nil {
		share = &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: "share1",
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
	hpv, err := hp.createHostpathVolume("volID", targetPath, volCtx, share, 0, mountAccess)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "namespace",
		},
	}

	cache.UpdateShare(share)

	cache.UpsertSecret(secret)
	err = hp.mapVolumeToPod(hpv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return secret
}

func primeConfigMapVolume(hp *hostPath, targetPath string, share *sharev1alpha1.Share, t *testing.T) *corev1.ConfigMap {
	volCtx := seedVolumeContext()
	if share == nil {
		share = &sharev1alpha1.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: "share1",
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
	hpv, err := hp.createHostpathVolume("volID", targetPath, volCtx, share, 0, mountAccess)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap1",
			Namespace: "namespace",
		},
	}

	cache.UpdateShare(share)

	cache.UpsertConfigMap(cm)
	err = hp.mapVolumeToPod(hpv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return cm
}

func findSharedItems(dir string, t *testing.T) (bool, bool) {
	foundSecret := false
	foundConfigMap := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		t.Logf("found file %s dir flag %v", info.Name(), info.IsDir())
		if err == nil && strings.Contains(info.Name(), "secret1") {
			foundSecret = true
		}
		if err == nil && strings.Contains(info.Name(), "configmap1") {
			foundConfigMap = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected walk error: %s", err.Error())
	}
	return foundSecret, foundConfigMap
}
