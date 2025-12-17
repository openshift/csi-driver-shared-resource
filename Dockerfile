FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.25-openshift-4.21 AS builder
WORKDIR /go/src/github.com/openshift/csi-driver-shared-resource
# to make SAST/SNYK happy
RUN rm -rf examples
RUN rm -f vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml
COPY . .
RUN rm -rf /go/src/github.com/openshift/csi-driver-shared-resource/examples
RUN rm -f /go/src/github.com/openshift/csi-driver-shared-resource/vendor/k8s.io/apimachinery/pkg/util/managedfields/pod.yaml
RUN make build

FROM registry.ci.openshift.org/ocp/4.21:base-rhel9
COPY --from=builder /go/src/github.com/openshift/csi-driver-shared-resource/_output/csi-driver-shared-resource /usr/bin/
ENTRYPOINT []
CMD ["/usr/bin/csi-driver-shared-resource"]
