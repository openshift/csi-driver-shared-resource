// +build norefresh

package e2e

import (
	"fmt"
	"testing"

	"github.com/openshift/csi-driver-shared-resource/test/framework"
)

func TestChangeRefreshConfigurationThenBasic(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)

	testArgs.CurrentDriverContainerRestartCount = framework.GetPodContainerRestartCount(testArgs)
	framework.TurnOffRefreshResources(testArgs)
	err := framework.WaitForPodContainerRestart(testArgs)
	if err != nil {
		testArgs.MessageString = fmt.Sprintf("hostpath container restart did not seem to occur")
		framework.LogAndDebugTestError(testArgs)
	}

	testArgs.SearchString = "Refresh-Resources disabled"
	framework.SearchCSIPods(testArgs)

	basicShareSetupAndVerification(testArgs)
}
