FROM registry.access.redhat.com/ubi9/go-toolset@sha256:ead35188c5748efe2b9420352aba56b02b43d8fcd7e879cc96c6b9ac2548e454 AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -o openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.access.redhat.com/ubi9/ubi-micro@sha256:f6e0a71b7e0875b54ea559c2e0a6478703268a8d4b8bdcf5d911d0dae76aef51

COPY --from=builder /opt/app-root/src/openshift-builds-shared-resource-webhook .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource-webhook"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource-webhook" \
	name="openshift-builds/csi-driver-shared-resource-webhook" \
	version="v1.1.0" \
	summary="Red Hat OpenShift Builds Shared Resource Webhook" \
	maintainer="openshift-builds@redhat.com" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource Webhook" \
	io.openshift.tags="builds,shared-resources,webhook,csi-driver"