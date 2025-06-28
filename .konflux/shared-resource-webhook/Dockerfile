FROM registry.access.redhat.com/ubi9/go-toolset@sha256:3bbd87d77ea93742bd71a5275a31ec4a7693454ab80492c6a7d28ce6eef35378 AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -o openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.access.redhat.com/ubi9/ubi-micro@sha256:dca8bc186bb579f36414c6ad28f1dbeda33e5cf0bd5fc1c51430cc578e25f819

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