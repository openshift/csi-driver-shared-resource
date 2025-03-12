FROM registry.access.redhat.com/ubi9/go-toolset@sha256:6402ad7886fb3bdaa950c89bdf338f8dbedde54c16bddb1157ebb8691b2def50 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.access.redhat.com/ubi9-minimal@sha256:30bbd445046a3a63f5f5557a3c67dee74e3c8e7855eb0347630b020f3689823f

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