# Installing the Projected Resource CSI driver

## Before you begin

1. You must have an OpenShift cluster running 4.8 or later.

1. Grant `cluster-admin` permissions to the current user.

## Installation (developer preview)

### Installing from the release page

Run the following command, providing an available release version.
Available versions can be found on the [releases page](https://github.com/openshift/csi-driver-projected-resource/releases).

```bash
$ export RELEASE_VERSION="v0.4.8-rc.0"
$ oc apply -f "https://github.com/openshift/csi-driver-projected-resource/releases/download/${RELEASE_VERSION}/release.yaml"
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
namespace/csi-driver-projected-resource created
customresourcedefinition.apiextensions.k8s.io/shares.projectedresource.storage.openshift.io created
serviceaccount/csi-driver-shared-resource created
clusterrole.rbac.authorization.k8s.io/csi-driver-shared-resource created
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource-privileged created
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource created
csidriver.storage.k8s.io/csi-driver-projected-resource.openshift.io created
service/csi-driver-shared-resource created
daemonset.apps/csi-driver-shared-resource created
```

### Installing from a local clone of this repository

1. Run the following command

```bash
# change directories into you clone of this repository, then
./deploy/deploy.sh
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
deploying hostpath components
   ./deploy/0000_10_projectedresource.crd.yaml
oc apply -f ./deploy/0000_10_projectedresource.crd.yaml
customresourcedefinition.apiextensions.k8s.io/shares.projectedresource.storage.openshift.io created
   ./deploy/00-namespace.yaml
oc apply -f ./deploy/00-namespace.yaml
namespace/csi-driver-projected-resource created
   ./deploy/01-service-account.yaml
oc apply -f ./deploy/01-service-account.yaml
serviceaccount/csi-driver-shared-resource created
   ./deploy/02-cluster-role.yaml
oc apply -f ./deploy/02-cluster-role.yaml
clusterrole.rbac.authorization.k8s.io/csi-driver-shared-resource created
   ./deploy/03-cluster-role-binding.yaml
oc apply -f ./deploy/03-cluster-role-binding.yaml
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource-privileged unchanged
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource unchanged
   ./deploy/csi-hostpath-driverinfo.yaml
oc apply -f ./deploy/csi-hostpath-driverinfo.yaml
csidriver.storage.k8s.io/csi-driver-projected-resource.openshift.io created
   ./deploy/csi-hostpath-plugin.yaml
oc apply -f ./deploy/csi-hostpath-plugin.yaml
service/csi-driver-shared-resource created
daemonset.apps/csi-driver-shared-resource created
16:21:25 waiting for hostpath deployment to complete, attempt #0
```

### Installing from the default branch of this repository

1. Run the following command

```bash
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/00-namespace.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/0000_10_projectedresource.crd.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/01-service-account.yaml 
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/02-cluster-role.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/03-cluster-role-binding.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/csi-hostpath-driverinfo.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/master/deploy/csi-hostpath-plugin.yaml 
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
namespace/csi-driver-projected-resource created
customresourcedefinition.apiextensions.k8s.io/shares.projectedresource.storage.openshift.io created
serviceaccount/csi-driver-shared-resource created
clusterrole.rbac.authorization.k8s.io/csi-driver-shared-resource created
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource-privileged created
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource created
csidriver.storage.k8s.io/csi-driver-projected-resource.openshift.io created
service/csi-driver-shared-resource created
daemonset.apps/csi-driver-shared-resource created
```


### Installing from a release specific branch of this repository

1. Run the following command

```bash
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/00-namespace.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/0000_10_projectedresource.crd.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/01-service-account.yaml 
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/02-cluster-role.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/03-cluster-role-binding.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/csi-hostpath-driverinfo.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-projected-resource/release-4.8/deploy/csi-hostpath-plugin.yaml 
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
namespace/csi-driver-projected-resource created
customresourcedefinition.apiextensions.k8s.io/shares.projectedresource.storage.openshift.io created
serviceaccount/csi-driver-shared-resource created
clusterrole.rbac.authorization.k8s.io/csi-driver-shared-resource created
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource-privileged created
clusterrolebinding.rbac.authorization.k8s.io/csi-driver-shared-resource created
csidriver.storage.k8s.io/csi-driver-projected-resource.openshift.io created
service/csi-driver-shared-resource created
daemonset.apps/csi-driver-shared-resource created
```


## Validate the installation

Every node should have a pod running the driver plugin in the namespace `csi-driver-projected-resource`.
On a 3 node OCP cluster, this will look something like:

```shell
$ oc get pods -n csi-driver-projected-resource
NAME                       READY   STATUS    RESTARTS   AGE
csi-driver-shared-resource-c7bbk   2/2     Running   0          23m
csi-driver-shared-resource-m4smv   2/2     Running   0          23m
csi-driver-shared-resource-x9xjw   2/2     Running   0          23m
```
