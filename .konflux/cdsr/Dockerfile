FROM registry.access.redhat.com/ubi9/go-toolset:1.20.12 AS builder
WORKDIR /go/src/github.com/openshift/csi-driver-shared-resource
COPY . .
RUN rm -rf /go/src/github.com/openshift/csi-driver-shared-resource/examples
RUN rm -f /go/src/github.com/openshift/csi-driver-shared-resource/vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml
RUN make build

FROM registry.access.redhat.com/ubi9/ubi:9.4-1181
COPY --from=builder /go/src/github.com/openshift/csi-driver-shared-resource/_output/csi-driver-shared-resource /usr/bin/
ENTRYPOINT []
CMD ["/usr/bin/csi-driver-shared-resource"]
