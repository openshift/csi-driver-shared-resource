FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder
WORKDIR /go/src/github.com/openshift/csi-driver-projected-resource
COPY . .
RUN make build

FROM registry.svc.ci.openshift.org/origin/4.6:base
COPY --from=builder /go/src/github.com/openshift/csi-driver-projected-resource/_output/csi-driver-projected-resource /usr/bin/
ENTRYPOINT []
CMD ["/usr/bin/csi-driver-projected-resource"]