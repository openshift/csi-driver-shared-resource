module github.com/openshift/projected-resource-csi-driver

go 1.14

require (
	github.com/container-storage-interface/spec v1.3.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/kubernetes-csi/csi-lib-utils v0.7.0
	github.com/pborman/uuid v1.2.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	google.golang.org/grpc v1.31.0
	k8s.io/client-go v0.19.0-rc.3
	k8s.io/klog v1.0.0
	k8s.io/kubelet v0.19.0-rc.3
	k8s.io/utils v0.0.0-20200731180307-f00132d28269
)
