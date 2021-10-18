module github.com/openshift/csi-driver-shared-resource

go 1.14

require (
	github.com/container-storage-interface/spec v1.3.0
	github.com/kubernetes-csi/csi-lib-utils v0.7.0
	github.com/openshift/api v0.0.0-20211007134530-4cb30f221b89
	github.com/openshift/client-go v0.0.0-20211007143529-7ab6242249ff
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	google.golang.org/grpc v1.31.0
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubectl v0.21.2
	k8s.io/utils v0.0.0-20210707171843-4b05e18ac7d9
)
