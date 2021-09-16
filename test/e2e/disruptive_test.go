// +build disruptive

package e2e

import (
	"github.com/openshift/csi-driver-shared-resource/test/framework"
	"testing"
	"time"
)

func TestBasicThenDriverRestartThenChangeShare(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	basicShareSetupAndVerification(testArgs)

	t.Logf("%s: initiating csi driver restart", time.Now().String())
	framework.RestartDaemonSet(testArgs)
	t.Logf("%s: csi driver restart complete, check test pod", time.Now().String())
	testArgs.TestDuration = 30 * time.Second
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)

	t.Logf("%s: now changing share", time.Now().String())
	framework.ChangeShare(testArgs)
	testArgs.SearchString = ".dockerconfigjson"
	framework.ExecPod(testArgs)
}
