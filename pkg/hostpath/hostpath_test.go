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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/csi-driver-projected-resource/pkg/cache"
)

func testHostPathDriver() (*hostPath, string, error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		return nil, "", err
	}
	hp, err := NewHostPathDriver(tmpDir, "ut-driver", "nodeID1", "endpoint1", 0, "version1")
	return hp, tmpDir, err
}

func TestCreateHostPathVolumeBadAccessType(t *testing.T) {
	hp, dir, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir)
	_, err = hp.createHostpathVolume("volID", "podNamespace", "podName", "podUID",
		"podSA", "", 0, mountAccess+1)
	if err == nil {
		t.Fatalf("err nil unexpectedly")
	}
	if !strings.Contains(err.Error(), "unsupported access type") {
		t.Fatalf("unexpected err %s", err.Error())
	}
}

func TestCreateHostPathVolume(t *testing.T) {
	hp, dir, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	secret, cm := primeVolume(hp, targetPath, t)

	foundSecret, foundConfigMap := findSharedItems(targetPath, t)
	if !foundSecret {
		t.Fatalf("did not find secret in mount path")
	}
	if !foundConfigMap {
		t.Fatalf("did not find configmap in mount path")
	}

	cache.DelSecret(secret)
	cache.DelConfigMap(cm)

	foundSecret, foundConfigMap = findSharedItems(dir, t)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
}

func TestDeleteHostPathVolume(t *testing.T) {
	hp, dir, err := testHostPathDriver()
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.RemoveAll(dir)
	targetPath, err := ioutil.TempDir(os.TempDir(), "ut")
	if err != nil {
		t.Fatalf("err on targetPath %s", err.Error())
	}
	defer os.RemoveAll(targetPath)
	primeVolume(hp, targetPath, t)

	err = hp.deleteHostpathVolume("volID")
	if err != nil {
		t.Fatalf("unexpeted error on delete volume: %s", err.Error())
	}
	foundSecret, foundConfigMap := findSharedItems(dir, t)

	if foundSecret {
		t.Fatalf("secret not deleted")
	}
	if foundConfigMap {
		t.Fatalf("configmap not deleted")
	}
	if empty, err := isDirEmpty(dir); !empty || err != nil {
		t.Fatalf("volume directory not cleaned out empty %v err %s", empty, err.Error())
	}
}

func primeVolume(hp *hostPath, targetPath string, t *testing.T) (*corev1.Secret, *corev1.ConfigMap) {
	hpv, err := hp.createHostpathVolume("volID", "podNamespace", "podName", "podUID",
		"podSA", targetPath, 0, mountAccess)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "namespace",
		},
	}

	cache.UpsertSecret(secret)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap1",
			Namespace: "namespace",
		},
	}

	cache.UpsertConfigMap(cm)

	err = hp.mapVolumeToPod(hpv)
	if err != nil {
		t.Fatalf("unexpected err %s", err.Error())
	}
	return secret, cm
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
