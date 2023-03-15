//go:build normal
// +build normal

package e2e

import (
	"testing"
	"time"

	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
	"github.com/openshift/csi-driver-shared-resource/test/framework"
)

func TestNoRBAC(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	framework.CreateShare(testArgs)
	framework.CreateTestPod(testArgs)
}

func TestNoShare(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	framework.CreateShareRelatedRBAC(testArgs)
	framework.CreateTestPod(testArgs)
}

func coreTestBasicThenNoShareThenShare(testArgs *framework.TestArgs) {
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	basicShareSetupAndVerification(testArgs)

	testArgs.T.Logf("%s: deleting share for %s", time.Now().String(), testArgs.Name)

	testArgs.ShareToDeleteType = consts.ResourceReferenceTypeConfigMap
	framework.DeleteShare(testArgs)
	testArgs.TestDuration = 30 * time.Second
	testArgs.SearchStringMissing = true
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)

	testArgs.T.Logf("%s: adding share back for %s", time.Now().String(), testArgs.Name)

	framework.CreateShare(testArgs)
	testArgs.SearchStringMissing = false
	testArgs.SearchString = "invoker"
	framework.ExecPod(testArgs)
}

func TestBasicNoRefresh(t *testing.T) {
	testArgs := &framework.TestArgs{
		T:         t,
		NoRefresh: false,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	basicShareSetupAndVerification(testArgs)
}

func TestBasicGroupRBAC(t *testing.T) {
	testArgs := &framework.TestArgs{
		T:                 t,
		NoRefresh:         false,
		TestGroupRBACOnly: true,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	basicShareSetupAndVerification(testArgs)
}

func TestBasicThenNoShareThenShareReadWrite(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	coreTestBasicThenNoShareThenShare(testArgs)
}

func coreTestTwoSharesSeparateMountPaths(testArgs *framework.TestArgs) {
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	doubleShareSetupAndVerification(testArgs)
}

func TestTwoSharesSeparateMountPaths(t *testing.T) {
	testArgs := &framework.TestArgs{
		T: t,
	}
	coreTestTwoSharesSeparateMountPaths(testArgs)
}

func TestRejectPodWithReadOnlyFalseSharedVolume(t *testing.T) {
	testArgs := &framework.TestArgs{
		T:            t,
		TestReadOnly: true,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	framework.CreateShareRelatedRBAC(testArgs)
	framework.CreateShare(testArgs)
	framework.CreateTestPod(testArgs)
}

func TestRejectShareWithOpenShiftPrefix(t *testing.T) {
	testArgs := &framework.TestArgs{
		T:                       t,
		ShareNameOverride:       "openshift-foo",
		TestShareCreateRejected: true,
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	framework.CreateShareRelatedRBAC(testArgs)
	framework.CreateShare(testArgs)
}

func TestAcceptShareWithOpenShiftPrefix(t *testing.T) {
	testArgs := &framework.TestArgs{
		T:                 t,
		ShareNameOverride: "openshift-etc-pki-entitlement",
	}
	prep(testArgs)
	framework.CreateTestNamespace(testArgs)
	defer framework.CleanupTestNamespaceAndClusterScopedResources(testArgs)
	framework.CreateShareRelatedRBAC(testArgs)
	framework.CreateReservedOpenShiftShare(testArgs)
}

/* a consequence of read only volume mounts is that we no longer support inherited mount paths
across separate shares in that mode.  The sub path mount encounters read only file system errors.

In the pod attempting to use a share whose mount path is under another share's mount path :

"Error: container create failed: time=\"2021-05-17T21:46:49Z\" level=error msg=\"container_linux.go:367: starting container process caused:
process_linux.go:495: container init caused:
rootfs_linux.go:60: mounting \\\"/var/lib/kubelet/pods/e3a70800-8d62-400e-854b-b1a02fc0e14f/volumes/kubernetes.io~csi/my-csi-volume-second-share/mount\\\"
to rootfs at \\\"/var/lib/containers/storage/overlay/9a2c6dad956e911bd02c369d0cbd013312b514dee81993913769ac81d248b565/merged/data/data-second-share\\\"
caused: mkdir /var/lib/containers/storage/overlay/9a2c6dad956e911bd02c369d0cbd013312b514dee81993913769ac81d248b565/merged/data/data-second-share: read-only file system\"\n"

*/
