
oc login https://api.ci.l2s4.p1.openshiftapps.com:6443
oc registry login

podman login quay.io

make build-image -e REGISTRY=quay.io -e REPOSITORY=akram
make deploy -e DRIVER_IMAGE=quay.io/akram/origin-csi-driver-shared-resource:latest

oc create namespace my-csi-app-namespace 
oc create cm -n my-csi-app-namespace my-other-share --from-file=cm/


oc get pods -n openshift-cluster-csi-drivers  -o wide

oc apply -f examples/

