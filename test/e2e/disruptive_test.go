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

	t.Logf("initiating csi driver restart")
	framework.RestartDaemonSet(t)
	t.Logf("csi driver restart complete, check test pod")
	framework.ExecPod(testNS, "openshift-config:openshift-install", false, 30*time.Second, t)

	t.Logf("now changing share")
	framework.ChangeShare(testNS, t)
	framework.ExecPod(testNS, "openshift-config:pull-secret", false, 30*time.Second, t)
}
