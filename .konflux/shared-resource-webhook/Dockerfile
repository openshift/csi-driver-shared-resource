FROM registry.redhat.io/ubi10/go-toolset@sha256:321516119f1b2487e114ee09a8ae29cfa81c959449a0fc63c8c94e1a2dcf69f5 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.redhat.io/ubi10-minimal@sha256:c769ab6615ab32d02d5b3d65ff2eae6bc668ba823e77f46b78a4181f54d17a27

WORKDIR /

COPY --from=builder /tmp/openshift-builds-shared-resource-webhook .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource-webhook"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource-webhook" \
	cpe="cpe:/a:redhat:openshift_builds:1.9::el10" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	distribution-scope="public" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource Webhook" \
	io.openshift.tags="builds,shared-resources,webhook,csi-driver" \
	maintainer="openshift-builds@redhat.com" \
	name="openshift-builds/openshift-builds-shared-resource-webhook-rhel10" \
	release="3" \
	summary="Red Hat OpenShift Builds Shared Resource Webhook" \
	url="https://github.com/openshift/csi-driver-shared-resource" \
	vendor="Red Hat, Inc." \
	version="v1.9.2"
