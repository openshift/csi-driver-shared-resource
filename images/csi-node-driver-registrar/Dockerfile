#FROM alpine
#LABEL maintainers="Kubernetes Authors"
#LABEL description="HostPath Driver"
#ARG binary=./bin/hostpathplugin

# Add util-linux to get a new version of losetup.
#RUN apk add util-linux
#COPY ${binary} /hostpathplugin
#ENTRYPOINT ["/hostpathplugin"]
FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder
WORKDIR /go/src/github.com/openshift/projected-resource-csi-driver
COPY . .
RUN make build

FROM registry.svc.ci.openshift.org/origin/4.6:base
COPY --from=builder /go/src/github.com/openshift/projected-resource-csi-driver/_output/csi-node-driver-registrar /usr/bin/
ENTRYPOINT []
CMD ["/usr/bin/csi-node-driver-registrar"]