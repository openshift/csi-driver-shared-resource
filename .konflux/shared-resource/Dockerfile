FROM registry.redhat.io/ubi9/go-toolset@sha256:23c7cb17e9c53a65f9ab01f2edf1a4e07c9a9dc2258ed63d69e06925e7cea381 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource ./cmd/csidriver

FROM registry.redhat.io/ubi9-minimal@sha256:08d4c70f1baf6537a0728656cf7942d561e74a35916509d205b6519861266352

RUN \
  microdnf --assumeyes --nodocs install util-linux && \
  microdnf clean all && \
  rm -rf /var/cache/yum

WORKDIR /

COPY --from=builder /tmp/openshift-builds-shared-resource .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource" \
	cpe="cpe:/a:redhat:openshift_builds:1.8::el9" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	distribution-scope="public" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource" \
	io.openshift.tags="builds,shared-resources,csi-driver" \
	maintainer="openshift-builds@redhat.com" \
	name="openshift-builds/openshift-builds-shared-resource-rhel9" \
	release="5" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	url="https://github.com/openshift/csi-driver-shared-resource" \
	vendor="Red Hat, Inc." \
	version="v1.8.4"
