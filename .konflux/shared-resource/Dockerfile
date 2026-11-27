FROM registry.redhat.io/ubi10/go-toolset@sha256:b9f867772f02ef0c8167b1ac327b615c9c6b06be16f5a42ea322597633d1d25e AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource ./cmd/csidriver

FROM registry.redhat.io/ubi10-minimal@sha256:d63890a635f8f19177bce1775f74f587a47f4456844f1392456c4e3673a48753

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
	cpe="cpe:/a:redhat:openshift_builds:1.9::el10" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	distribution-scope="public" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource" \
	io.openshift.tags="builds,shared-resources,csi-driver" \
	maintainer="openshift-builds@redhat.com" \
	name="openshift-builds/openshift-builds-shared-resource-rhel10" \
	release="4" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	url="https://github.com/openshift/csi-driver-shared-resource" \
	vendor="Red Hat, Inc." \
	version="v1.9.3"
