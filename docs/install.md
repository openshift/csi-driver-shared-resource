# Installing the Shared Resource CSI driver

## Before you begin

1. You must have an OpenShift cluster running 4.8 or later.

1. Grant `cluster-admin` permissions to the current user.

## Installation (developer preview)

### Installing from the release page

Run the following command, providing an available release version.
Available versions can be found on the [releases page](https://github.com/openshift/csi-driver-shared-resource/releases).

```bash
$ export RELEASE_VERSION="v0.4.8-rc.0"
$ oc apply -f "https://github.com/openshift/csi-driver-shared-resource/releases/download/${RELEASE_VERSION}/release.yaml"
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
customresourcedefinition.apiextensions.k8s.io/sharedsecrets.sharedresource.openshift.io created
customresourcedefinition.apiextensions.k8s.io/sharedconfigmaps.sharedresource.openshift.io created
serviceaccount/csi-driver-shared-resource-plugin created
clusterrole.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-privileged created
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
csidriver.storage.k8s.io/csi-driver-shared-resource.openshift.io created
service/shared-resource-csi-driver-node created
daemonset.apps/shared-resource-csi-driver-node created
```

### Installing from a local clone of this repository

1. Run the following command

```bash
# change directories into you clone of this repository, then
make deploy
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
deploying hostpath components
   vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedconfigmap.crd.yaml
oc apply -f vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedconfigmap.crd.yaml
customresourcedefinition.apiextensions.k8s.io/sharedconfigmaps.sharedresource.openshift.io created
deploying hostpath components
   vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedsecret.crd.yaml
oc apply -f vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedsecret.crd.yaml
customresourcedefinition.apiextensions.k8s.io/sharedsecrets.sharedresource.openshift.io created
   ./deploy/00-namespace.yaml
oc apply -f ./deploy/00-namespace.yaml
namespace/openshift-cluster-csi-drivers unchanged
   ./deploy/01-service-account.yaml
oc apply -f ./deploy/01-service-account.yaml
serviceaccount/csi-driver-shared-resource-plugin created
   ./deploy/02-cluster-role.yaml
oc apply -f ./deploy/02-cluster-role.yaml
clusterrole.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
   ./deploy/03-cluster-role-binding.yaml
oc apply -f ./deploy/03-cluster-role-binding.yaml
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-privileged unchanged
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create unchanged
   ./deploy/csi-hostpath-driverinfo.yaml
oc apply -f ./deploy/csi-hostpath-driverinfo.yaml
csidriver.storage.k8s.io/csi.sharedresource.openshift.io created
   ./deploy/csi-hostpath-plugin.yaml
oc apply -f ./deploy/csi-hostpath-plugin.yaml
service/shared-resource-csi-driver-node created
daemonset.apps/shared-resource-csi-driver-node created
16:21:25 waiting for hostpath deployment to complete, attempt #0
```

### Installing from the default branch of this repository

1. Run the following command

```bash
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/deploy/00-namespace.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedconfigmap.crd.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedsecret.crd.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/deploy/01-service-account.yaml 
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/deploy/02-cluster-role.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/deploy/03-cluster-role-binding.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/deploy/csi-hostpath-driverinfo.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/master/deploy/csi-hostpath-plugin.yaml 
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
namespace/csi-driver-shared-resource created
customresourcedefinition.apiextensions.k8s.io/sharedconfigmaps.sharedresource.openshift.io created
customresourcedefinition.apiextensions.k8s.io/sharedsecrets.sharedresource.openshift.io created
serviceaccount/csi-driver-shared-resource-plugin created
clusterrole.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-privileged created
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
csidriver.storage.k8s.io/csi.sharedresource.openshift.io created
service/shared-resource-csi-driver-node created
daemonset.apps/shared-resource-csi-driver-node created
```


### Installing from a release specific branch of this repository

1. Run the following command

```bash
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/deploy/00-namespace.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedconfigmap.crd.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/vendor/github.com/openshift/api/sharedresource/v1alpha1/0000_10_sharedsecret.crd.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/deploy/01-service-account.yaml 
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/deploy/02-cluster-role.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/deploy/03-cluster-role-binding.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/deploy/csi-hostpath-driverinfo.yaml
oc apply -f --filename https://raw.githubusercontent.com/openshift/csi-driver-shared-resource/release-4.10/deploy/csi-hostpath-plugin.yaml 
```

You should see an output similar to the following printed on the terminal showing the creation or modification of the various
Kubernetes resources:

```shell
namespace/openshift-cluster-csi-drivers unchanged
customresourcedefinition.apiextensions.k8s.io/sharedconfigmaps.sharedresource.openshift.io created
customresourcedefinition.apiextensions.k8s.io/sharedsecrets.sharedresource.openshift.io created
serviceaccount/csi-driver-shared-resource-plugin created
clusterrole.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-privileged created
clusterrolebinding.rbac.authorization.k8s.io/shared-resource-secret-configmap-share-watch-sar-create created
csidriver.storage.k8s.io/csi.sharedresource.openshift.io created
service/shared-resource-csi-driver-node created
daemonset.apps/shared-resource-csi-driver-node created
```


## Validate the installation

Every node should have a pod running the driver plugin in the namespace `openshift-cluster-csi-drivers`.
On a 3 node OCP cluster, this will look something like:

```shell
$ oc get pods -n openshift-cluster-csi-drivers
NAME                       READY   STATUS    RESTARTS   AGE
shared-resource-csi-driver-node-c7bbk   2/2     Running   0          23m
shared-resource-csi-driver-node-m4smv   2/2     Running   0          23m
shared-resource-csi-driver-node-x9xjw   2/2     Running   0          23m
```
