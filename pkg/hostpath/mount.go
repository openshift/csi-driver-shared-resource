package hostpath

import (
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/utils/mount"
)

type FileSystemMounter interface {
	makeFSMounts(volumeRequest *csi.NodePublishVolumeRequest, hostPathDriver HostPathDriver, mounter mount.Interface) error
	removeFSMounts(intermediateBindMountDir, kubeletTargetDir string, mount mount.Interface) error
}

// ReadWriteMany high level details:
//
// This is our original landing spot wrt mounting the file system this driver manipulates
// to where the location the kubelet has allocated for the CSI volume in question.
//
// We go straight from our tmpfs anchor to kubelet's target directory.  No bind mounts.
// But this approach only works if the K8s CSIVolumeSource set readOnly to false.  If readOnly
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

func (m *ReadWriteMany) makeFSMounts(volumeRequest *csi.NodePublishVolumeRequest, hostPathDriver HostPathDriver, mounter mount.Interface) error {
	options := []string{}
	kubeletTargetDir := volumeRequest.GetTargetPath()
	anchorDir, _ := hostPathDriver.getVolumePath(volumeRequest.GetVolumeId(), volumeRequest.GetVolumeContext())
	if err := mounter.Mount(anchorDir, kubeletTargetDir, TmpFsType, options); err != nil {
		var errList strings.Builder
		errList.WriteString(err.Error())
		if rmErr := os.RemoveAll(anchorDir); rmErr != nil && !os.IsNotExist(rmErr) {
			errList.WriteString(fmt.Sprintf(" :%s", rmErr.Error()))
		}

		return status.Error(codes.Internal, fmt.Sprintf("failed to mount device: %s at %s: %s",
			AnchorDir,
			kubeletTargetDir,
			errList.String()))
	}
	return nil
}

func (m *ReadWriteMany) removeFSMounts(intermediateBindMountDir, kubeletTargetDir string, mounter mount.Interface) error {
	if err := mount.CleanupMountPoint(kubeletTargetDir, mounter, true); err != nil {
		return err
	}
	if err := os.RemoveAll(AnchorDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// WriteOnceReadMany details:
// Our second generation version after ReadWriteMany.
// Using an intermediate bind mount between our anchor / starting point and the kubelet's target directory, this
// can write updates to this intermediate directory, and see those updates reflected in the Pod's volume, even though
// it is read only for any code running in the Pod.
// However, to date, we cannot restart this driver and support updates to the volume surfacing to the Pod.  The Pod's
// view of the volume still has whatever content was present before the driver restart.
// On the driver restart, the anchorDir and intermediateBindMountDir tmpfs dirs are no longer present.  And recreation
// and remount to the kubeletTargetDir is not sufficient to facilitate future updates being visible from the Pod.
//
// But the bind mount intermediate layer facilitates use of tmpfs for CSIVolumeSource with readOnly set to true.
type WriteOnceReadMany struct {
}

func (m *WriteOnceReadMany) makeFSMounts(volumeRequest *csi.NodePublishVolumeRequest, hostPathDriver HostPathDriver, mounter mount.Interface) error {
	options := []string{}
	kubeletTargetDir := volumeRequest.GetTargetPath()
	anchorDir, bindDir := hostPathDriver.getVolumePath(volumeRequest.GetVolumeId(), volumeRequest.GetVolumeContext())
	if err := mounter.Mount(anchorDir, bindDir, "tmpfs", options); err != nil {
		var errList strings.Builder
		errList.WriteString(err.Error())
		if rmErr := os.RemoveAll(anchorDir); rmErr != nil && !os.IsNotExist(rmErr) {
			errList.WriteString(fmt.Sprintf(" :%s", rmErr.Error()))
		}

		return status.Error(codes.Internal, fmt.Sprintf("failed to mount device: %s at %s: %s",
			anchorDir,
			bindDir,
			errList.String()))
	}

	// now add bind and ro options
	options = append(options, "bind", "ro")

	if err := mounter.Mount(bindDir, kubeletTargetDir, TmpFsType, options); err != nil {
		var errList strings.Builder
		errList.WriteString(err.Error())
		if rmErr := os.RemoveAll(bindDir); rmErr != nil && !os.IsNotExist(rmErr) {
			errList.WriteString(fmt.Sprintf(" :%s", rmErr.Error()))
		}

		return status.Error(codes.Internal, fmt.Sprintf("failed to mount device: %s at %s: %s",
			bindDir,
			kubeletTargetDir,
			errList.String()))
	}

	return nil
}

func (m *WriteOnceReadMany) removeFSMounts(intermediateBindMountDir, kubeletTargetDir string, mounter mount.Interface) error {
	if err := mount.CleanupMountPoint(kubeletTargetDir, mounter, true); err != nil {
		return err
	}
	if err := mount.CleanupMountPoint(intermediateBindMountDir, mounter, true); err != nil {
		return err
	}
	if err := os.RemoveAll(intermediateBindMountDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(AnchorDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
