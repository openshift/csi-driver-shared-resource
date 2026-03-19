FROM registry.redhat.io/ubi9/go-toolset@sha256:8b211cc8793d8013140844e8070aa584a8eb3e4e6e842ea4b03dd9914b1a5dc6 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource-webhook ./cmd/webhook

FROM registry.redhat.io/ubi9-minimal@sha256:f225a0f8eb7d258011e1f7c8b025dc7dd155d6ffc66c17cd203dcec88935b455

WORKDIR /

COPY --from=builder /tmp/openshift-builds-shared-resource-webhook .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource-webhook"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource-webhook" \
	name="openshift-builds/openshift-builds-shared-resource-webhook-rhel9" \
	version="v1.6.1" \
	summary="Red Hat OpenShift Builds Shared Resource Webhook" \
	maintainer="openshift-builds@redhat.com" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource Webhook" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource Webhook" \
	io.openshift.tags="builds,shared-resources,webhook,csi-driver" \
	cpe="cpe:/a:redhat:openshift_builds:1.6::el9"	\
	distribution-scope="public" \
  url="https://catalog.redhat.com/en/software/container-stacks/detail/6565a473b9526bf84dae4d85" \
  vendor="Red Hat, Inc." \
  release="1.6.1"
