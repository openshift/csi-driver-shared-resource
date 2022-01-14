package hostpath

import (
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/utils/mount"
)

type FileSystemMounter interface {
	makeFSMounts(mountIDString, intermediateBindMountDir, kubeletTargetDir string, mounter mount.Interface) error
	removeFSMounts(mountIDString, intermediateBindMountDir, kubeletTargetDir string, mount mount.Interface) error
}

// ReadWriteMany high level details:
//
// This is our original landing spot wrt mounting the file system this driver manipulates
// to where the location the kubelet has allocated for the CSI volume in question.
//
// We go straight from our "identifier" string based on input from Jan to the kubelet's target directory.  No bind mounts.
// But this approach only works if the K8s CSIVolumenSource set readOnly to false.  If readOnly
// is set to true, the underlying mount mechanics between our call to m.mounter.Mount and what
// the kubelet does for the Pod results in the use of xfs for the filesystem and an inability for the
// Pod to read what we have mounted.
//
// Additional details:
//
// So our intent here is to have a separate tmpfs per pod; through experimentation
// and corroboration with OpenShift storage SMEs, a separate tmpfs per pod
// - ensures the kubelet will handle SELinux for us. It will relabel the volume in "the right way" just for the pod
// - otherwise, if pods share the same host dir, all sorts of warnings from the SMEs
// - and the obvious isolation between pods that implies
// We cannot do read-only on the mount since we have to copy the data after the mount, otherwise we get errors
// that the filesystem is readonly.
// However, we can restart this driver, leave up any live Pods with our volume, and then still update the content
// after this driver comes backup.
// The various bits that work in concert to achieve this
// - the use of emptyDir with a medium of Memory in this drivers Deployment is all that is needed to get tmpfs
// - do not use the "bind" option, that reuses existing dirs/filesystems vs. creating new tmpfs
// - without bind, we have to specify an fstype of tmpfs and path for the mount source, or we get errors on the
//   Mount about the fs not being  block access
// - that said,  testing confirmed using fstype of tmpfs on hostpath/xfs volumes still results in the target
//   being xfs and not tmpfs
// - with the lack of a bind option, and each pod getting its own tmpfs we have to copy the data from our emptydir
//   based location to the targetPath here ... that is handled in hostpath.go
type ReadWriteMany struct {
}

func (m *ReadWriteMany) makeFSMounts(mountIDString, intermediateBindMountDir, kubeletTargetDir string, mounter mount.Interface) error {
	options := []string{}
	if err := mounter.Mount(mountIDString, kubeletTargetDir, "tmpfs", options); err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to mount device: %s at %s: %s",
			mountIDString,
			kubeletTargetDir,
			err.Error()))
	}
	return nil
}

func (m *ReadWriteMany) removeFSMounts(mountIDString, intermediateBindMountDir, kubeletTargetDir string, mounter mount.Interface) error {
	// mount.CleanupMountPoint proved insufficient for us, as it always considered our mountIDString here "not a mount", even
	// though we would rsh into the driver container/pod and manually run 'umount'.  If we did not do this, then
	// the termination of pods using our CSI driver could hang.  So we just directly call Unmount from out mounter.
	if err := mounter.Unmount(mountIDString); err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to umount device: %s at %s and %s: %s",
			mountIDString,
			intermediateBindMountDir,
			kubeletTargetDir,
			err.Error()))
	}
	return nil
}
