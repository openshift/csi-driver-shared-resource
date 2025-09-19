FROM registry.access.redhat.com/ubi9/go-toolset@sha256:8a9e1465ae487fa2b4a94d2baa249a8d59fb3d7e6884a7b22c783f494f4cfb01 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.access.redhat.com/ubi9-minimal@sha256:7c5495d5fad59aaee12abc3cbbd2b283818ee1e814b00dbc7f25bf2d14fa4f0c

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