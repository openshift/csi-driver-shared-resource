FROM registry.redhat.io/ubi9/go-toolset:1.24 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource ./cmd/csidriver

FROM registry.access.redhat.com/ubi9-minimal:9.6

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
	name="openshift-builds/openshift-builds-shared-resource-rhel9" \
	version="v1.6.0" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	maintainer="openshift-builds@redhat.com" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource" \
	io.openshift.tags="builds,shared-resources,csi-driver"\
    cpe="cpe:/a:redhat:openshift_builds:1.6::el9"
