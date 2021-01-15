package framework

import "testing"

// LogAndDebugTestError is not intended as a replacement for the use of t.Fatalf through this e2e suite,
// but when errors occur that could benefit from a dump of the CSI Driver pod logs, use this method instead
// of simply calling t.Fatalf
func LogAndDebugTestError(msg string, t *testing.T) {
	t.Logf("*** TEST %s FAILED BEGIN OF CSI DRIVER POD DUMP", t.Name())
	dumpPod(t)
	t.Logf("*** TEST %s FAILED END OF CSI DRIVER POD DUMP", t.Name())
	t.Fatalf(msg)
}
