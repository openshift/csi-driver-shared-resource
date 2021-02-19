// +build normal

package e2e

import (
	"github.com/openshift/csi-driver-projected-resource/test/framework"
	"testing"
	"time"
)

func TestNoRBAC(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	framework.CreateShare(testArgs)
	framework.CreateTestPod(testArgs)
}

func TestNoShare(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	framework.CreateShareRelatedRBAC(testArgs)
	framework.CreateTestPod(testArgs)
}

func TestBasicThenNoShareThenShare(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	basicShareSetupAndVerification(testArgs)

	t.Logf("%s: deleting share for %s", time.Now().String(), testArgs.Name)

	framework.DeleteShare(testArgs)
	testArgs.TestDuration = 30 * time.Second
	testArgs.SearchStringMissing = true
	testArgs.SearchString = "openshift-config:openshift-install"
	framework.ExecPod(testArgs)
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)

	t.Logf("%s: adding share back for %s", time.Now().String(), testArgs.Name)

	framework.CreateShare(testArgs)
	testArgs.SearchStringMissing = false
	testArgs.SearchString = "openshift-config:openshift-install"
	framework.ExecPod(testArgs)
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)
}

func TestTwoSharesSeparateMountPaths(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	doubleShareSetupAndVerification(testArgs)
}

func TestTwoSharesSeparateButInheritedMountPaths(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	testArgs.SecondShareSubDir = true
	doubleShareSetupAndVerification(testArgs)
}

func TestTwoSharesSeparateButInheritedMountPathsRemoveSubPath(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	testArgs.SecondShareSubDir = true
	doubleShareSetupAndVerification(testArgs)

	testArgs.ShareToDelete = testArgs.SecondName
	framework.DeleteShare(testArgs)
	testArgs.SearchString = "openshift-config:openshift-install"
	testArgs.TestDuration = 30 * time.Second
	framework.ExecPod(testArgs)
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)

	testArgs.SearchStringMissing = true
	testArgs.SearchString = "openshift-config:pull-secret"
	framework.ExecPod(testArgs)
	testArgs.SearchString = ".dockerconfigjson"
	framework.ExecPod(testArgs)
}

func TestTwoSharesSeparateButInheritedMountPathsRemoveTopPath(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespace(testArgs)
	testArgs.SecondShareSubDir = true
	doubleShareSetupAndVerification(testArgs)

	framework.DeleteShare(testArgs)
	testArgs.TestDuration = 30 * time.Second
	testArgs.SearchString = "openshift-config:pull-secret"
	framework.ExecPod(testArgs)
	testArgs.SearchString = ".dockerconfigjson"
	framework.ExecPod(testArgs)

	testArgs.SearchStringMissing = true
	testArgs.SearchString = "openshift-config:openshift-install"
	framework.ExecPod(testArgs)
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)
}
