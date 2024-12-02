FROM registry.access.redhat.com/ubi9/go-toolset@sha256:45170b6e45114849b5d2c0e55d730ffa4a709ddf5f58b9e810548097b085e78f AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -o openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.access.redhat.com/ubi9/ubi-micro@sha256:a410623c2b8e9429f9606af821be0231fef2372bd0f5f853fbe9743a0ddf7b34

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