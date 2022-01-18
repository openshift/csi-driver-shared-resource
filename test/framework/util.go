package framework

import (
	"testing"
	"time"

	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
)

var (
	secondShareSuffix = "-second-share"
)

type TestArgs struct {
	T                                  *testing.T
	Name                               string
	SecondName                         string
	ChangeName                         string
	SearchString                       string
	MessageString                      string
	ShareToDelete                      string
	LogContent                         string
	CurrentDriverContainerRestartCount map[string]int32
	ShareToDeleteType                  consts.ResourceReferenceType
	SearchStringMissing                bool
	SecondShare                        bool
	SecondShareSubDir                  bool
	DaemonSetUp                        bool
	TestPodUp                          bool
	NoRefresh                          bool
	TestDuration                       time.Duration
}

// LogAndDebugTestError is not intended as a replacement for the use of t.Fatalf through this e2e suite,
// but when errors occur that could benefit from a dump of the CSI Driver pod logs, use this method instead
// of simply calling t.Fatalf
func LogAndDebugTestError(t *TestArgs) {
	t.T.Logf("*** TEST %s FAILED BEGIN OF CSI DRIVER POD DUMP at time %s", t.T.Name(), time.Now().String())
	dumpCSIPods(t)
	t.T.Logf("*** TEST %s FAILED END OF CSI DRIVER POD DUMP at time %s", t.T.Name(), time.Now().String())
	t.T.Logf("*** TEST %s FAILED BEGIN OF TEST POD DUMP at time %s", t.T.Name(), time.Now().String())
	dumpTestPod(t)
	t.T.Logf("*** TEST %s FAILED END OF TEST POD DUMP at time %s", t.T.Name(), time.Now().String())
	t.T.Logf("*** TEST %s FAILED BEGIN OF TEST EVENT DUMP at time %s", t.T.Name(), time.Now().String())
	dumpTestPodEvents(t)
	t.T.Logf("*** TEST %s FAILED END OF TEST EVENT DUMP at time %s", t.T.Name(), time.Now().String())
	t.T.Fatalf(t.MessageString)
}
