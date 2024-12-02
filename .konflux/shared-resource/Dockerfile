FROM registry.access.redhat.com/ubi9/go-toolset@sha256:b23212b6a296e95cbed5dd415d4cd5567db9e8057aa08e291f1fddc087cea944 AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -ldflags="-s -w" -o openshift-builds-shared-resource ./cmd/csidriver

FROM registry.access.redhat.com/ubi9@sha256:1057dab827c782abcfb9bda0c3900c0966b5066e671d54976a7bcb3a2d1a5e53

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
