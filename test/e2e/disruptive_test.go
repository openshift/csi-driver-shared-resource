// +build disruptive

package e2e

import (
	"github.com/openshift/csi-driver-projected-resource/test/framework"
	"testing"
	"time"
)

func TestBasicThenDriverRestartThenChangeShare(t *testing.T) {
	prep(t)
	testNS := framework.CreateTestNamespace(t)
	defer framework.CleanupTestNamespace(testNS, t)
	basicShareSetupAndVerification(testNS, t)

	t.Logf("%s: initiating csi driver restart", time.Now().String())
	framework.RestartDaemonSet(t)
	t.Logf("%s: csi driver restart complete, check test pod", time.Now().String())
	framework.ExecPod(testNS, "openshift-config:openshift-install", false, 30*time.Second, t)

	t.Logf("%s: now changing share", time.Now().String())
	framework.ChangeShare(testNS, t)
	framework.ExecPod(testNS, "openshift-config:pull-secret", false, 30*time.Second, t)
}
