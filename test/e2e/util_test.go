package e2e

import (
	"fmt"
	"time"

	"github.com/openshift/csi-driver-projected-resource/test/framework"
)

func prep(t *framework.TestArgs) {
	framework.SetupClients(t)
	framework.LaunchDriver(t)
	t.DaemonSetUp = true
	err := framework.WaitForDaemonSet(t)
	if err != nil {
		t.MessageString = fmt.Sprintf("csi driver daemon not up: %s", err.Error())
		framework.LogAndDebugTestError(t)
	}
}

func basicShareSetupAndVerification(t *framework.TestArgs) {
	framework.CreateShareRelatedRBAC(t)
	framework.CreateShare(t)
	t.TestPodUp = true
	framework.CreateTestPod(t)
	t.TestDuration = 30 * time.Second
	t.SearchString = "invoker"
	framework.ExecPod(t)

}

func doubleShareSetupAndVerification(t *framework.TestArgs) {
	t.SecondShare = true
	basicShareSetupAndVerification(t)
	t.SearchString = ".dockerconfigjson"
	framework.ExecPod(t)

}
