package e2e

import (
	"fmt"
	"time"

	"github.com/openshift/csi-driver-shared-resource/test/framework"
)

func prep(t *framework.TestArgs) {
	framework.SetupClientsOutsideTestNamespace(t)
	t.DaemonSetUp = true
	t.WebhookUp = true
	err := framework.WaitForDaemonSet(t)
	if err != nil {
		t.MessageString = fmt.Sprintf("csi driver daemon not up: %s", err.Error())
		framework.LogAndDebugTestError(t)
	}
	err = framework.WaitForWebhook(t)
	if err != nil {
		t.MessageString = fmt.Sprintf("csi driver webhook not up: %s", err.Error())
		framework.LogAndDebugTestError(t)
	}
}

func basicShareSetupAndVerification(t *framework.TestArgs) {
	framework.CreateShareRelatedRBAC(t)
	framework.CreateDriverRBAC(t, "openshift-config", "configmaps", "openshift-install")
	framework.CreateShare(t)
	t.TestPodUp = true
	framework.CreateTestPod(t)
	t.TestDuration = 30 * time.Second
	t.SearchString = "invoker"
	framework.ExecPod(t)

}

func doubleShareSetupAndVerification(t *framework.TestArgs) {
	t.SecondShare = true
	framework.CreateDriverRBAC(t, "openshift-config", "secrets", "pull-secret")
	basicShareSetupAndVerification(t)
	t.SearchString = ".dockerconfigjson"
	framework.ExecPod(t)

}
