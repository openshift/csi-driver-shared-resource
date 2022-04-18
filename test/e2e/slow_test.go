//go:build slow
// +build slow

package e2e

import (
	"github.com/openshift/csi-driver-shared-resource/test/framework"
	"testing"
	"time"
)

// this requires up to a 22 minute delay for 2 separate relist
func TestBasicThenNoRBACThenRBAC(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	basicShareSetupAndVerification(testArgs)

	framework.DeleteShareRelatedRBAC(testArgs)
	t.Logf("%s: wait up to 11 minutes for examining pod %s since the controller does not currently watch all clusterroles and clusterrolebindings and reverse engineer which ones satisfied the SAR calls, so we wait for relist on shares", time.Now().String(), testArgs.Name)
	testArgs.SearchStringMissing = true
	testArgs.TestDuration = 11 * time.Minute
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)

	framework.CreateShareRelatedRBAC(testArgs)
	t.Logf("%s: wait up to 11 minutes for examining pod %s since the controller does not currently watch all clusterroles and clusterrolebindings and reverse engineer which ones satisfied the SAR calls, so we wait for relist on shares", time.Now().String(), testArgs.Name)
	testArgs.SearchStringMissing = false
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)
}
