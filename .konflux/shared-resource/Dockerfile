FROM registry.access.redhat.com/ubi9/go-toolset@sha256:45170b6e45114849b5d2c0e55d730ffa4a709ddf5f58b9e810548097b085e78f AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -ldflags="-s -w" -o openshift-builds-shared-resource ./cmd/csidriver

FROM registry.access.redhat.com/ubi9@sha256:d342aa80781bf41c4c73485c41d8f1e2dbc40ee491633d9cafe787c361dd44ff

COPY --from=builder /opt/app-root/src/openshift-builds-shared-resource .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource" \
	name="openshift-builds/csi-driver-shared-resource" \
	version="v1.1.0" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	maintainer="openshift-builds@redhat.com" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource" \
	io.openshift.tags="builds,shared-resources,csi-driver"
