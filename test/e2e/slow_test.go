// +build slow

package e2e

import (
	"github.com/openshift/csi-driver-projected-resource/test/framework"
	"testing"
	"time"
)

// this requires up to a 20 minute delay for 2 separate relist
func TestBasicThenNoRBACThenRBAC(t *testing.T) {
	prep(t)
	testNS := framework.CreateTestNamespace(t)
	defer framework.CleanupTestNamespace(testNS, t)
	basicShareSetupAndVerification(testNS, t)

	framework.DeleteShareRelatedRBAC(testNS, t)
	t.Logf("%s: wait up to 10 minutes for examining pod %s since the controller does not currently watch all clusterroles and clusterrolebindings and reverse engineer which ones satisfied the SAR calls, so we wait for relist on shares", time.Now().String(), testNS)
	framework.ExecPod(testNS, "openshift-config:openshift-install", true, 10*time.Minute, t)

	framework.CreateShareRelatedRBAC(testNS, t)
	t.Logf("%s: wait up to 10 minutes for examining pod %s since the controller does not currently watch all clusterroles and clusterrolebindings and reverse engineer which ones satisfied the SAR calls, so we wait for relist on shares", time.Now().String(), testNS)
	framework.ExecPod(testNS, "openshift-config:openshift-install", false, 10*time.Minute, t)
}
