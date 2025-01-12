FROM registry.access.redhat.com/ubi9/go-toolset@sha256:6ec9c3ce36c929ff98c1e82a8b7fb6c79df766d1ad8009844b59d97da9afed43 AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -buildvcs=false -ldflags="-s -w" -o openshift-builds-shared-resource ./cmd/csidriver

FROM registry.access.redhat.com/ubi9@sha256:38791b293262ac2169eca2717e68e626a047d2b89fbd1da544db24ed0204efeb

COPY --from=builder /opt/app-root/src/openshift-builds-shared-resource .
COPY LICENSE /licenses/

ENTRYPOINT ["./openshift-builds-shared-resource"]

LABEL \
	com.redhat.component="openshift-builds-shared-resource" \
	name="openshift-builds/csi-driver-shared-resource" \
	version="v1.1.0" \
	summary="Red Hat OpenShift Builds Shared Resource" \
	maintainer="openshift-builds@redhat.com" \
	description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.description="Red Hat OpenShift Builds CSI Driver Shared Resource" \
	io.k8s.display-name="Red Hat OpenShift Builds Shared Resource" \
	io.openshift.tags="builds,shared-resources,csi-driver"
