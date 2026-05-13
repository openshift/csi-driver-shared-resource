FROM registry.redhat.io/ubi9/go-toolset@sha256:e06a6f4c85c3ca75f64127542449c9770fb885adfb592f987c576d268ac108de AS builder

ENV GOEXPERIMENT=strictfipsruntime

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=1 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -tags="strictfipsruntime" -o /tmp/openshift-builds-shared-resource ./cmd/csidriver

FROM registry.redhat.io/ubi9-minimal@sha256:12db9874bd753eb98b1ab3d840e75de5d6842ac0604fbd68c012adefe97140be

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
	name="openshift-builds/csi-driver-shared-resource" \
	release="1" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	url="https://github.com/openshift/csi-driver-shared-resource" \
	vendor="Red Hat, Inc." \
	version="v1.8.0"
