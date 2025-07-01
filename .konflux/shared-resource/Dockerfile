FROM registry.access.redhat.com/ubi9/go-toolset@sha256:94ed5700c85822759e23cb1ddfa42f2485a56e709106e7060905588f62c46f22 AS builder

COPY . .

RUN rm -f /vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -mod=vendor -ldflags="-s -w" -o openshift-builds-shared-resource ./cmd/csidriver

FROM registry.access.redhat.com/ubi9@sha256:ea57285741f007e83f2ee20423c20b0cbcce0b59cc3da027c671692cc7efe4dd

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
