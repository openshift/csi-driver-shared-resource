/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hostpath

import (
	"fmt"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
	"github.com/openshift/csi-driver-projected-resource/pkg/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
)

const (
	TopologyKeyNode           = "topology.hostpath.csi/node"
	CSIPodName                = "csi.storage.k8s.io/pod.name"
	CSIPodNamespace           = "csi.storage.k8s.io/pod.namespace"
	CSIPodUID                 = "csi.storage.k8s.io/pod.uid"
	CSIPodSA                  = "csi.storage.k8s.io/serviceAccount.name"
	CSIEphemeral              = "csi.storage.k8s.io/ephemeral"
	ProjectedResourceShareKey = "share"
)

var (
	listers client.Listers
)

type nodeServer struct {
	nodeID            string
	maxVolumesPerNode int64
	hp                HostPathDriver
	mounter           mount.Interface
}

func NewNodeServer(hp *hostPath) *nodeServer {
	return &nodeServer{
		nodeID:            hp.nodeID,
		maxVolumesPerNode: hp.maxVolumesPerNode,
		hp:                hp,
		mounter:           mount.New(""),
	}
}

func getPodDetails(volumeContext map[string]string) (string, string, string, string) {
	podName, _ := volumeContext[CSIPodName]
	podNamespace, _ := volumeContext[CSIPodNamespace]
	podSA, _ := volumeContext[CSIPodSA]
	podUID, _ := volumeContext[CSIPodUID]
	return podNamespace, podName, podUID, podSA

}

func (ns *nodeServer) validateShare(req *csi.NodePublishVolumeRequest) (*sharev1alpha1.Share, error) {
	shareName, sok := req.GetVolumeContext()[ProjectedResourceShareKey]
	if !sok || len(strings.TrimSpace(shareName)) == 0 {
		return nil, status.Errorf(codes.InvalidArgument,
			"the csi driver reference is missing the volumeAttribute 'share'")
	}

	share, err := client.GetListers().Shares.Get(shareName)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			"the csi driver volumeAttribute 'share' reference had an error: %s", err.Error())
	}

	switch strings.TrimSpace(share.Spec.BackingResource.Kind) {
	case "Secret":
	case "ConfigMap":
	default:
		return nil, status.Errorf(codes.InvalidArgument,
			"the share %s has an invalid backing resource kind %s", shareName, share.Spec.BackingResource.Kind)
	}

	if len(strings.TrimSpace(share.Spec.BackingResource.Namespace)) == 0 {
		return nil, status.Errorf(codes.InvalidArgument,
			"the share %s backing resource namespace needs to be set", shareName)
	}
	if len(strings.TrimSpace(share.Spec.BackingResource.Name)) == 0 {
		return nil, status.Errorf(codes.InvalidArgument,
			"the share %s backing resource name needs to be set", shareName)
	}

	podNamespace, podName, _, podSA := getPodDetails(req.GetVolumeContext())

	allowed, err := client.ExecuteSAR(shareName, podNamespace, podName, podSA)
	if allowed {
		return share, nil
	}
	return nil, err
}

// validateVolumeContext return values:
func (ns *nodeServer) validateVolumeContext(req *csi.NodePublishVolumeRequest) error {

	podNamespace, podName, podUID, podSA := getPodDetails(req.GetVolumeContext())
	klog.V(4).Infof("NodePublishVolume pod %s ns %s sa %s uid %s",
		podName,
		podNamespace,
		podSA,
		podUID)

	if len(podName) == 0 || len(podNamespace) == 0 || len(podUID) == 0 || len(podSA) == 0 {
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("Volume attributes missing required set for pod: namespace: %s name: %s uid: %s, sa: %s",
				podNamespace, podName, podUID, podSA))
	}
	ephemeralVolume := req.GetVolumeContext()[CSIEphemeral] == "true" ||
		req.GetVolumeContext()[CSIEphemeral] == "" // Kubernetes 1.15 doesn't have csi.storage.k8s.io/ephemeral.

	if !ephemeralVolume {
		return status.Error(codes.InvalidArgument, "Non-ephemeral request made")
	}

	if req.GetVolumeCapability().GetMount() == nil {
		return status.Error(codes.InvalidArgument, "only support mount access type")
	}
	return nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	var targetPath string

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if req.VolumeContext == nil || len(req.GetVolumeContext()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume attributes missing in request")
	}

	err := ns.validateVolumeContext(req)
	if err != nil {
		return nil, err
	}

	share, err := ns.validateShare(req)
	if err != nil {
		return nil, err
	}

	targetPath = req.GetTargetPath()
	vol, err := ns.hp.createHostpathVolume(req.GetVolumeId(), targetPath, req.GetVolumeContext(), share, maxStorageCapacity, mountAccess)
	if err != nil && !os.IsExist(err) {
		klog.Error("ephemeral mode failed to create volume: ", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.V(4).Infof("NodePublishVolume created volume: %s", vol.VolPath)

	notMnt, err := mount.IsNotMountPoint(ns.mounter, targetPath)

	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// this means the mount.Mounter call has already happened
	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	fsType := req.GetVolumeCapability().GetMount().GetFsType()

	deviceId := ""
	if req.GetPublishContext() != nil {
		deviceId = req.GetPublishContext()[deviceID]
	}

	volumeId := req.GetVolumeId()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

	klog.V(4).Infof("NodePublishVolume %v\nfstype %v\ndevice %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
		targetPath, fsType, deviceId, volumeId, attrib, mountFlags)

	options := []string{}
	path := vol.VolPath

	// NOTE: so our intent here is to have a separate tmpfs per pod; through experimentation
	// and corroboration with OpenShift storage SMEs, a separate tmpfs per pod
	// - ensures the kubelet will handle SELinux for us. It will relabel the volume in "the right way" just for the pod
	// - otherwise, if pods share the same host dir, all sorts of warnings from the SMEs
	// - and the obvious isolation between pods that implies
	// We cannot do read-only on the mount since we have to copy the data after the mount, otherwise we get errors
	// that the filesystem is readonly
	// The various bits that work in concert to achieve this
	// - the use of emptyDir with a medium of Memory in this drivers Deployment is all that is needed to get tmpfs
	// - do not use the "bind" option, that reuses existing dirs/filesystems vs. creating new tmpfs
	// - without bind, we have to specify an fstype of tmpfs and path for the mount source, or we get errors on the
	//   Mount about the fs not being  block access
	// - that said,  testing confirmed using fstype of tmpfs on hostpath/xfs volumes still results in the target
	//   being xfs and not tmpfs
	// - with the lack of a bind option, and each pod getting its own tmpfs we have to copy the data from our emptydir
	//   based location to the targetPath here ... that is handled in hostpath.go
	if err := ns.mounter.Mount(path, targetPath, "tmpfs", options); err != nil {
		var errList strings.Builder
		errList.WriteString(err.Error())
		if rmErr := os.RemoveAll(path); rmErr != nil && !os.IsNotExist(rmErr) {
			errList.WriteString(fmt.Sprintf(" :%s", rmErr.Error()))
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mount device: %s at %s: %s",
			path,
			targetPath,
			errList.String()))
	}
	// here is what initiates that necessary copy now with *NOT* using bind on the mount so each pod gets its own tmpfs
	if err := ns.hp.mapVolumeToPod(vol); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to populate mount device: %s at %s: %s",
			path,
			targetPath,
			err.Error()))
	}

	if err := storeVolMapToDisk(); err != nil {
		klog.Errorf("failed to persist driver volume metadata to disk: %s", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	err := mount.CleanupMountPoint(targetPath, ns.mounter, true)
	if err != nil {
		klog.Errorf("error cleaning and unmounting target path %s, err: %v for vol: %s", targetPath, err, volumeID)
	}

	klog.V(4).Infof("hostpath: volume %s has been unpublished.", targetPath)

	klog.V(4).Infof("deleting volume %s", volumeID)
	if err := ns.hp.deleteHostpathVolume(volumeID); err != nil && !os.IsNotExist(err) {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete volume: %s", err))
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {

	topology := &csi.Topology{
		Segments: map[string]string{TopologyKeyNode: ns.nodeID},
	}

	return &csi.NodeGetInfoResponse{
		NodeId:             ns.nodeID,
		MaxVolumesPerNode:  ns.maxVolumesPerNode,
		AccessibleTopology: topology,
	}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{},
	}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeExpandVolume is only implemented so the driver can be used for e2e testing.
func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
