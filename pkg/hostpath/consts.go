package hostpath

type accessType int

const (
	deviceID                          = "deviceID"
	storageCapacityKB      int64      = 1024
	storageCapacityMB                 = storageCapacityKB * 1024
	storageCapacityGB                 = storageCapacityMB * 1024
	storageCapacityTB                 = storageCapacityGB * 1024
	maxStorageCapacity                = storageCapacityTB
	TopologyKeyNode                   = "topology.hostpath.csi/node"
	CSIPodName                        = "csi.storage.k8s.io/pod.name"
	CSIPodNamespace                   = "csi.storage.k8s.io/pod.namespace"
	CSIPodUID                         = "csi.storage.k8s.io/pod.uid"
	CSIPodSA                          = "csi.storage.k8s.io/serviceAccount.name"
	CSIEphemeral                      = "csi.storage.k8s.io/ephemeral"
	SharedResourceShareKey            = "sharedresource"
	anchorDir                         = "anchor-dir"
	bindDir                           = "bind-dir"
	mountAccess            accessType = iota
)
