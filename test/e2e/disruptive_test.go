// +build disruptive

package e2e

import (
	"testing"
	"time"

	"github.com/openshift/csi-driver-shared-resource/test/framework"
)

func inner(testArgs *framework.TestArgs, t *testing.T) {
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	basicShareSetupAndVerification(testArgs)

	t.Logf("%s: initiating csi driver restart", time.Now().String())
	framework.RestartDaemonSet(testArgs)
	t.Logf("%s: csi driver restart complete, check test pod", time.Now().String())
	testArgs.TestDuration = 30 * time.Second
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)

	testArgs.ChangeName = "kube-root-ca.crt"
	t.Logf("%s: now changing share to %s", time.Now().String(), testArgs.ChangeName)
	framework.ChangeShare(testArgs)
	testArgs.SearchString = "ca.crt"
	framework.ExecPod(testArgs)
}

func TestBasicThenDriverRestartThenChangeShare(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	for i := 0; i < 3; i++ {
		inner(testArgs, t)
	}
}
