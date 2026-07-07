FROM registry.redhat.io/ubi10/go-toolset@sha256:d5d48915a31c7c774caf7568f7fbe3b25275e042f9f4de73d13fba39f9b2a987 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.redhat.io/ubi10-minimal@sha256:2c20ac20ca1ecbbbd583603feabd9cc51e7e8ea5a82e5088e20a9494794b2574

WORKDIR /

COPY --from=builder /tmp/openshift-builds-shared-resource-webhook .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource-webhook"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource-webhook" \
	cpe="cpe:/a:redhat:openshift_builds:1.8::el10" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	distribution-scope="public" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource Webhook" \
	io.openshift.tags="builds,shared-resources,webhook,csi-driver" \
	maintainer="openshift-builds@redhat.com" \
	name="openshift-builds/csi-driver-shared-resource-webhook" \
	release="1" \
	summary="Red Hat OpenShift Builds Shared Resource Webhook" \
	url="https://github.com/openshift/csi-driver-shared-resource" \
	vendor="Red Hat, Inc." \
	version="v1.8.0"
