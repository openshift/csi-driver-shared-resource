// +build normal

package e2e

import (
	"github.com/openshift/csi-driver-projected-resource/test/framework"
	"testing"
	"time"
)

func TestNoRBAC(t *testing.T) {
	prep(t)
	testNS := framework.CreateTestNamespace(t)
	defer framework.CleanupTestNamespace(testNS, t)
	framework.CreateShare(testNS, t)
	framework.CreateTestPod(testNS, false, t)
}

func TestNoShare(t *testing.T) {
	prep(t)
	testNS := framework.CreateTestNamespace(t)
	defer framework.CleanupTestNamespace(testNS, t)
	framework.CreateShareRelatedRBAC(testNS, t)
	framework.CreateTestPod(testNS, false, t)
}

func TestBasicThenNoShareThenShare(t *testing.T) {
	prep(t)
	testNS := framework.CreateTestNamespace(t)
	defer framework.CleanupTestNamespace(testNS, t)
	basicShareSetupAndVerification(testNS, t)

	t.Logf("deleting share for %s", testNS)

	framework.DeleteShare(testNS, t)
	framework.ExecPod(testNS, "openshift-config:openshift-install", true, 30*time.Second, t)

	t.Logf("adding share back for %s", testNS)

	framework.CreateShare(testNS, t)
	framework.ExecPod(testNS, "openshift-config:openshift-install", false, 30*time.Second, t)
}
