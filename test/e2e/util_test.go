package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/openshift/csi-driver-projected-resource/test/framework"
)

func prep(t *testing.T) {
	framework.SetupClients(t)
	framework.LaunchDriver(t)
	err := framework.WaitForDaemonSet(true, t)
	if err != nil {
		framework.LogAndDebugTestError(fmt.Sprintf("csi driver daemon not up: %s", err.Error()), t)
	}
}

func basicShareSetupAndVerification(name string, t *testing.T) {
	framework.CreateShareRelatedRBAC(name, t)
	framework.CreateShare(name, t)
	framework.CreateTestPod(name, true, t)
	framework.ExecPod(name, "openshift-config:openshift-install", false, 30*time.Second, t)

}
