FROM registry.redhat.io/ubi10/go-toolset@sha256:c468f485f6bdc8ba6b106d63d4019851eaf1912182f7e67ff1a185234d58ebf7 AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource ./cmd/csidriver

FROM registry.redhat.io/ubi10-minimal@sha256:04febb4a74cc9ef3eca05ef851d92957276cc6e82fe8cb1ee44abf5114d440d8

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
	name="openshift-builds/csi-driver-shared-resource" \
	release="2" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	url="https://github.com/openshift/csi-driver-shared-resource" \
	vendor="Red Hat, Inc." \
	version="v1.9.1"
