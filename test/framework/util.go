package framework

import (
	"testing"
	"time"
)

// LogAndDebugTestError is not intended as a replacement for the use of t.Fatalf through this e2e suite,
// but when errors occur that could benefit from a dump of the CSI Driver pod logs, use this method instead
// of simply calling t.Fatalf
func LogAndDebugTestError(msg string, t *testing.T) {
	t.Logf("*** TEST %s FAILED BEGIN OF CSI DRIVER POD DUMP at time %s", t.Name(), time.Now().String())
	dumpPod(t)
	t.Logf("*** TEST %s FAILED END OF CSI DRIVER POD DUMP at time %s", t.Name(), time.Now().String())
	t.Fatalf(msg)
}

//TODO presumably this can go away once we have an OLM based deploy that is also integrated with our CI
// so that repo images built from PRs are used when setting up this driver's daemonset
func LaunchDriver(t *testing.T) {
	CreateCSIDriverPlugin(t)
}
