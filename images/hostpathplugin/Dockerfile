FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder
WORKDIR /go/src/github.com/openshift/projected-resource-csi-driver
COPY . .
RUN make build

FROM registry.svc.ci.openshift.org/origin/4.6:base
COPY --from=builder /go/src/github.com/openshift/projected-resource-csi-driver/_output/hostpathplugin /usr/bin/
ENTRYPOINT []
CMD ["/usr/bin/hostpathplugin"]