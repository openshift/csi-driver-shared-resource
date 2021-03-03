<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [OpenShift Projected Resource "CSI DRIVER"](#openshift-projected-resource-csi-driver)
  - [Current status with respect to the Kubernetes CSIVolumeSource API](#current-status-with-respect-to-the-kubernetes-csivolumesource-api)
  - [Current status with respect to the Kubernetes VolumeMount API](#current-status-with-respect-to-the-kubernetes-volumemount-api)
  - [Current status with respect to the enhancement propsal](#current-status-with-respect-to-the-enhancement-propsal)
    - [Vetted scenarios](#vetted-scenarios)
    - [Excluded OCP namespaces](#excluded-ocp-namespaces)
  - [Deployment](#deployment)
  - [Run example application and validate](#run-example-application-and-validate)
  - [Confirm openshift-config ConfigMap data is present](#confirm-openshift-config-configmap-data-is-present)
  - [Reference to our blog on using this component to leverage RHEL Entitlements](#reference-to-our-blog-on-using-this-component-to-leverage-rhel-entitlements)
  - [Some more detail and examples around those vetted scenarios](#some-more-detail-and-examples-around-those-vetted-scenarios)
    - [What happens exactly if the Share is not there when you create a Pod that references it](#what-happens-exactly-if-the-share-is-not-there-when-you-create-a-pod-that-references-it)
    - [What happens if you have a long running Pod, and after starting it with the Share present, you remove the Share](#what-happens-if-you-have-a-long-running-pod-and-after-starting-it-with-the-share-present-you-remove-the-share)
    - [What happens if the ClusterRole or ClusterRoleBinding are not present when your newly created Pod tries to access an existing Share](#what-happens-if-the-clusterrole-or-clusterrolebinding-are-not-present-when-your-newly-created-pod-tries-to-access-an-existing-share)
    - [What happens if you have a long running Pod, and after starting it with the Share present and the necessary permissions present, you remove those permissions](#what-happens-if-you-have-a-long-running-pod-and-after-starting-it-with-the-share-present-and-the-necessary-permissions-present-you-remove-those-permissions)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# OpenShift Projected Resource "CSI DRIVER"

The Work In Progress implementation for [this OpenShift Enhancement Proposal](https://github.com/openshift/enhancements/blob/master/enhancements/cluster-scope-secret-volumes/csi-driver-host-injections.md),
this repository borrows from thes reference implementations:

- [CSI Hostpath Driver](https://github.com/kubernetes-csi/csi-driver-host-path)
- [Kubernetes-Secrets-Store-CSI-Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver)

As part of forking these into this repository, function not required for the projected resources scenarios have 
been removed, and the images containing these commands are built off of RHEL based images instead of the non-Red Hat
images used upstream.

As the enhancement proposal reveals, this is not a fully compliant CSI Driver implementation.  This repository
solely provides the minimum amounts of the Kubernetes / CSI contract needed to achieve the goals stipulated in the 
Enhancement proposal.

## Current status with respect to the Kubernetes CSIVolumeSource API

So let's take each part of the [CSIVolumeSource](https://github.com/kubernetes/api/blob/71efbb18d63cd30604981514ac623a6be1d413bb/core/v1/types.go#L1743-L1771):

- for the `Driver` string field, it needs to be ["csi-driver-projected-resource.openshift.io"](https://github.com/openshift/csi-driver-projected-resource/blob/1fcc354faa31f624086265ea2228661a0fc2e7b1/pkg/client/client.go#L28).
- for the `VolumeAttributes` map, this driver currently adds the "share" key (which maps the the `Share` instance your `Pod` wants to use) in addition to the 
elements of the `Pod` the kubelet stores when contacting the driver to provision the `Volume`.  See [this list](https://github.com/openshift/csi-driver-projected-resource/blob/c3f1c454f92203f4b406dabe8dd460782cac1d03/pkg/hostpath/nodeserver.go#L37-L42).
- the `ReadOnly` field is ignored, as the this driver's controller actively updates the `Volume` as the underlying `Secret` or `ConfigMap` change, or as 
the `Share` or the RBAC related to the `Share` change.
- the `FSType` field is ignored.  This driver by design only supports `tmpfs`, with a different mount performed for each `Volume`, in order to defer all SELinux concerns to the kubelet.
- the `NodePublishSecretRef` field is ignored.  The CSI `NodePublishVolume` and `NodeUnpublishVolume` flows gate the permission evaluation required for the `Volume`
by performing `SubjectAccessReviews` against the reference `Share` instance, using the `serviceAccount` of the `Pod` as the subject.
  
## Current status with respect to the Kubernetes VolumeMount API

So let's take each part of the [VolumeMount](https://github.com/openshift/csi-driver-projected-resource/blob/c3f1c454f92203f4b406dabe8dd460782cac1d03/pkg/hostpath/nodeserver.go#L37-L42),
where each `Container` in a `Pod` is allowed an array of such mounts:

- the validations that Kubernetes applies to `Name` and `MountPath` fields apply.
- if the `MountPath` of one `VolumeMount` is a subdirectory of another `MountPath` in the `Container`, Kubernetes will allow it, and this driver 
currently does support that scenario.
- if the `MountPath` corresponds to an existing directory with content from the image, or a directory that is populated as
  part of `Pod` setup outside this driver, the results cannot be guaranteed, and thus is unsupported.  Existing directories 
  from the image must be empty. 
- the `ReadOnly` field is currently ignored, per our explanation above with [CSIVolumeSource](#current-status-with-respect-to-the-kubernetes-csivolumesource-api).
- the `SubPath` field is currently ignored.
- the `MountPropagation` field is currently ignored, and is non-applicable to this driver, given that the data written to the `tmpfs` filesystem of the `Pod` comes
from the Kubernetes Controller cache, and not the host.  The empty directory constraints noted above for `MountPath` are related.
- the `SubPathExpr` field is currently ignored.

## Current status with respect to the enhancement propsal

**Developer Preview level of support pending.  Waiting on install via OLM from the OperatorHub (a feature currently under
development) to officially declare that the function provided in this repository has reached 
Developer Preview status.  To clarify how Developer Preview works with the various operators at the OpenShift OperatorHub,
Developer Preview releases are not intended to be run in production environments nor will the product be supported via the 
Red Hat Customer Portal case management system. If you need assistance, open issues on this GitHub repository and those issues 
will be addressed on a best effort basis.**

The latest commit of the master branch solely introduces both the `Share` CRD and the `projectedresoure.storage.openshift.io`
API group and version `v1alpha1`.  

The reference to the `share` object in the `volumeAttributes` in a declared CSI volume within a `Pod` is used to 
fuel a `SubjectAccessReview` check.  The `ServiceAccount` for the `Pod` must have `get` access to the `Share` in
order for the referenced `ConfigMap` and `Secret` to be mounted in the `Pod`.

A controller exists for watching this new CRD, as well as `ConfigMaps` and `Secrets` in all Namespaces except for
a list of OpenShift "system" namespaces which have `ConfigMaps` that get updated every few seconds.

Some high level remaining work:

- Revisit monitoring for permission changes as they happen in addition to checking on the re-list
- Monitoring of the prometheus metrics and pprof type ilk
- Configuration around which namespaces are and are not monitored
- Install via OLM and the OLM Operator Hub
  

### Vetted scenarios
 
The controller and CSI driver in their current form facilitate the following scenarios:

- initial pod requests for `Share` csi volumes are denied without both a valid `Share` refrence and 
permissions to access that `Share`
- changes to the `Share`'s backing resource (kind, namespace, name) get reflected in data stored in the user pod's CSI volume
- subsequent removal of permissions for a `Share` results in removal of the associated data stored in the user pod's CSI volume
- re-granting of permission for a `Share` (after having the permissions initially, then removed) results in the associated 
data getting stored in the user pod's CSI volume
- removal of the `Share` used to provision `Share` csi volume for a pod result in the associated data getting removed
- re-creation of a removed `Share` for a previously provisioned `Share` CSI volume results in the associated data 
reappearing in the user pod's CSI volume
- support recycling of the csi driver so that previously provisioned CSI volumes are still managed; in other words,
the driver's interan state is persisted 
- when multiple `Share`s are mounted in a `Pod`, one `Share` can be mounted as a subdirectory of another `Share`

### Excluded OCP namespaces

The current list of namespaces excluded from the controller's watches:

- kube-system
- openshift-machine-api
- openshift-kube-apiserver
- openshift-kube-apiserver-operator
- openshift-kube-scheduler
- openshift-kube-controller-manager
- openshift-kube-controller-manager-operator
- openshift-kube-scheduler-operator
- openshift-console-operator
- openshift-controller-manager
- openshift-controller-manager-operator
- openshift-cloud-credential-operator
- openshift-authentication-operator
- openshift-service-ca
- openshift-kube-storage-version-migrator-operator
- openshift-config-operator
- openshift-etcd-operator
- openshift-apiserver-operator
- openshift-cluster-csi-drivers
- openshift-cluster-storage-operator
- openshift-cluster-version
- openshift-image-registry
- openshift-machine-config-operator
- openshift-sdn
- openshift-service-ca-operator

The list is not yet configurable, but as noted above, most likely will become so as the project's lifecycle progresses.

## Deployment

Untile the OLM based install is available, the means to use the Projected Resource driver is to run the `deploy.sh` in the
`deploy` subdirectory of this repository.

```
# deploys csi projectedresource driver, RBAC related resources, namespace, Share CRD
$ deploy/deploy.sh
```

You should see an output similar to the following printed on the terminal showing the application of rbac rules and the
result of deploying the hostpath driver, external provisioner, external attacher and snapshotter components. Note that the following output is from Kubernetes 1.17:

```shell
deploying hostpath components
   deploy/hostpath/00-namespace.yaml
kubectl apply -f deploy/hostpath/00-namespace.yaml
namespace/csi-driver-projected-resource created
   deploy/hostpath/01-service-account.yaml
kubectl apply -f deploy/hostpath/01-service-account.yaml
serviceaccount/csi-driver-projected-resource-plugin created
   deploy/hostpath/03-cluster-role-binding.yaml
kubectl apply -f deploy/hostpath/03-cluster-role-binding.yaml
clusterrolebinding.rbac.authorization.k8s.io/projected-resource-privileged created
   deploy/hostpath/csi-hostpath-driverinfo.yaml
kubectl apply -f deploy/hostpath/csi-hostpath-driverinfo.yaml
csidriver.storage.k8s.io/csi-driver-projected-resource.openshift.io created
   deploy/hostpath/csi-hostpath-plugin.yaml
kubectl apply -f deploy/hostpath/csi-hostpath-plugin.yaml
service/csi-hostpathplugin created
daemonset.apps/csi-hostpathplugin created
```

## Run example application and validate

First, let's validate the deployment.  Ensure all expected pods are running for the driver plugin, which in a 
3 node OCP cluster will look something like:

```shell
$ kubectl get pods -n csi-driver-projected-resource
NAME                       READY   STATUS    RESTARTS   AGE
csi-hostpathplugin-c7bbk   2/2     Running   0          23m
csi-hostpathplugin-m4smv   2/2     Running   0          23m
csi-hostpathplugin-x9xjw   2/2     Running   0          23m
```

Next, let's start up the simple test application.  From the root directory, deploy from the `./examples` directory the 
application `Pod`, along with the associated test namespace, `Share`, `ClusterRole`, and `ClusterRoleBinding` definitions
needed to illustrate the mounting of one of the API types (in this instance a `ConfigMap` from the `openshift-config`
namespace) into the `Pod`:

```shell
$ kubectl apply -f ./examples
namespace/my-csi-app-namespace created
clusterrole.rbac.authorization.k8s.io/projected-resource-my-share created
clusterrolebinding.rbac.authorization.k8s.io/projected-resource-my-share created
share.projectedresource.storage.openshift.io/my-share created
pod/my-csi-app created
```

Ensure the `my-csi-app` comes up in `Running` state.

Finally, if you want to validate the volume, inspect the application pod `my-csi-app`:

```shell
$ kubectl describe pods/my-csi-app -n my-csi-app-namespace 
Name:         my-csi-app
Namespace:    my-csi-app-namespace
Priority:     0
Node:         ip-10-0-158-10.us-west-2.compute.internal/10.0.158.10
Start Time:   Fri, 19 Feb 2021 12:01:03 -0500
Labels:       <none>
Annotations:  k8s.v1.cni.cncf.io/network-status:
                [{
                    "name": "",
                    "interface": "eth0",
                    "ips": [
                        "10.129.2.31"
                    ],
                    "default": true,
                    "dns": {}
                }]
              k8s.v1.cni.cncf.io/networks-status:
                [{
                    "name": "",
                    "interface": "eth0",
                    "ips": [
                        "10.129.2.31"
                    ],
                    "default": true,
                    "dns": {}
                }]
              openshift.io/scc: node-exporter
Status:       Running
IP:           10.129.2.31
IPs:
  IP:  10.129.2.31
Containers:
  my-frontend:
    Container ID:  cri-o://929553d4c8966ffb1f0234a91cbbbbab66be2fc189871249a3cf7082046dfee1
    Image:         quay.io/quay/busybox
    Image ID:      quay.io/quay/busybox@sha256:ffd944135bc9fe6573e82d4578c28beb6e3fec1aea988c38d382587c7454f819
    Port:          <none>
    Host Port:     <none>
    Command:
      sleep
      1000000
    State:          Running
      Started:      Fri, 19 Feb 2021 12:01:08 -0500
    Ready:          True
    Restart Count:  0
    Environment:    <none>
    Mounts:
      /data from my-csi-volume (rw)
      /var/run/secrets/kubernetes.io/serviceaccount from default-token-5fjcb (ro)
Conditions:
  Type              Status
  Initialized       True 
  Ready             True 
  ContainersReady   True 
  PodScheduled      True 
Volumes:
  my-csi-volume:
    Type:              CSI (a Container Storage Interface (CSI) volume source)
    Driver:            csi-driver-projected-resource.openshift.io
    FSType:            
    ReadOnly:          false
    VolumeAttributes:      share=my-share
  default-token-5fjcb:
    Type:        Secret (a volume populated by a Secret)
    SecretName:  default-token-5fjcb
    Optional:    false
QoS Class:       BestEffort
Node-Selectors:  <none>
Tolerations:     node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                 node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
Events:
  Type    Reason          Age   From               Message
  ----    ------          ----  ----               -------
  Normal  Scheduled       65s   default-scheduler  Successfully assigned my-csi-app-namespace/my-csi-app to ip-10-0-158-10.us-west-2.compute.internal
  Normal  AddedInterface  64s   multus             Add eth0 [10.129.2.31/23]
  Normal  Pulling         63s   kubelet            Pulling image "quay.io/quay/busybox"
  Normal  Pulled          60s   kubelet            Successfully pulled image "quay.io/quay/busybox" in 2.920300054s
  Normal  Created         60s   kubelet            Created container my-frontend
  Normal  Started         60s   kubelet            Started container my-frontend
```


## Confirm openshift-config ConfigMap data is present

To verify, go back into the `Pod` named `my-csi-app` and list the contents:

  ```shell
  $ kubectl exec  -n my-csi-app-namespace -it my-csi-app /bin/sh
  / # ls -lR /data
  ```

You should see contents like:

```shell
/ # ls -lR /data 
ls -lR /data 
/data:
total 8
-rw-r--r--    1 root     root             4 Oct 28 14:52 invoker
-rw-r--r--    1 root     root            70 Oct 28 14:52 version
/ # 
```

**NOTE**: You'll notice that the driver has created subdirectories off of the `volumeMount` specified in our example `Pod`.
One subdirectory for the type (`configsmaps` or `secrets`), and one whose name is a concatenation of the `namespace` and
`name` of the `ConfigMap` or `Secret` being mounted.  As noted in the high level feature list above, new features that allow 
some control on how the files are laid out should be coming.

Now, if you inspect the contents of that `ConfigMap`, you'll see keys in the `data` map that 
correspond to the 2 files created:

```shell
$ oc get cm openshift-install -n openshift-config -o yaml
apiVersion: v1
data:
  invoker: user
  version: unreleased-master-3849-g9c8baf2f69c50a9d745d86f4784bdd6b426040af-dirty
kind: ConfigMap
metadata:
  creationTimestamp: "2020-10-28T13:30:47Z"
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:data:
        .: {}
        f:invoker: {}
        f:version: {}
    manager: cluster-bootstrap
    operation: Update
    time: "2020-10-28T13:30:47Z"
  name: openshift-install
  namespace: openshift-config
  resourceVersion: "1460"
  selfLink: /api/v1/namespaces/openshift-config/configmaps/openshift-install
  uid: 0382a47d-7c58-4198-b99e-eb3dc987da59
```

How `Secrets` and `ConfigMaps` are stored on disk mirror the storage for 
`Secrets` and `ConfigMaps` as done in the code in  [https://github.com/kubernetes/kubernetes](https://github.com/kubernetes/kubernetes)
where a file is created for each key in a `ConfigMap` `data` map or `binaryData` map and each key in a `Secret`
`data` map.

If you want to try other `ConfigMaps` or a `Secret`, first clear out the existing application:

```shell
$ oc delete -f ./examples 
``` 

And the edit `./examples/02-csi-share.yaml` and change the `backingResource` stanza to point to the item 
you want to share, and then re-run `oc apply -f ./examples`.

## Reference to our blog on using this component to leverage RHEL Entitlements

The blog post is under internal development.  A link to it will be added here when it is available.

## Some more detail and examples around those vetted scenarios

### What happens exactly if the Share is not there when you create a Pod that references it

You'll see an event like:

```bash
$ oc get events
0s          Warning   FailedMount      pod/my-csi-app                                       MountVolume.SetUp failed for volume "my-csi-volume" : rpc error: code = InvalidArgument desc = the csi driver volumeAttribute 'share' reference had an error: share.projectedresource.storage.openshift.io "my-share" not found
$
```

And your Pod will never reach the Running state.

However, if the kubelet is still in a retry cycle trying to launch a Pod with a `Share` reference, if `Share` non-existence is the only thing preventing a mount, the mount should then succeed if the `Share` comes into existence.

### What happens if you have a long running Pod, and after starting it with the Share present, you remove the Share

The data will be removed from the location specified by `volumeMount` in the `Pod`.  Instead of 

```bash
$ oc rsh my-csi-app
sh-4.4# ls -lR /data
ls -lR /data
total 312
-rw-r--r--. 1 root root   3243 Jan 29 17:59 4653723971430838710-key.pem
-rw-r--r--. 1 root root 311312 Jan 29 17:59 4653723971430838710.pem

```

You'll get 

```bash
oc rsh my-csi-app
sh-4.4# ls -lR /data
ls -lR /data
/data:
total 0
sh-4.4#

```

### What happens if the ClusterRole or ClusterRoleBinding are not present when your newly created Pod tries to access an existing Share

```bash
$ oc get events
LAST SEEN   TYPE      REASON        OBJECT           MESSAGE
6s          Normal    Scheduled     pod/my-csi-app   Successfully assigned my-csi-app-namespace/my-csi-app to ip-10-0-136-162.us-west-2.compute.internal
2s          Warning   FailedMount   pod/my-csi-app   MountVolume.SetUp failed for volume "my-csi-volume" : rpc error: code = PermissionDenied desc = subjectaccessreviews share my-share podNamespace my-csi-app-namespace podName my-csi-app podSA default returned forbidden
$

```
And your Pod will never get to the Running state.

### What happens if you have a long running Pod, and after starting it with the Share present and the necessary permissions present, you remove those permissions

The data will be removed from the `Pod’s` volumeMount location.

Instead of 

```bash
$ oc rsh my-csi-app
sh-4.4# ls -lR /data
ls -lR /data
/data:
total 312
-rw-r--r--. 1 root root   3243 Jan 29 17:59 4653723971430838710-key.pem
-rw-r--r--. 1 root root 311312 Jan 29 17:59 4653723971430838710.pem
sh-4.4#

```

You'll get 

```bash
oc rsh my-csi-app
sh-4.4# ls -lR /data
ls -lR /data
/data:
total 0
sh-4.4#
```

Do note that if your Pod copied the data to other locations, the Projected Resource driver cannot do anything about those copies.  A big motivator for allowing
some customization of the directory and file structure off of the `volumeMount` of the `Pod` is to help reduce the *need* to copy
files.  Hopefully you can mount that data directly at its final, needed, destination.

Also note that the Projected Resource does not try to reverse engineer which RoleBinding or ClusterRoleBinding allows your Pod to access the Share. 
The Kubernetes and OpenShift libraries for this are not currently structured to be openly consumed by other components.  Nor did we entertain taking 
snapshots of that code to serve such a purpose.  So instead of listening to RoleBinding or Role changes, on the Projected Resource controller’s re-list interval 
(which is configurable via start up argument on the command invoked from out DaemonSet, and whose default is 10 minutes), the controller will re-execute 
Subject Access Review requests for each Pod’s reference to each `Share` on the `Share` re-list and remove content if permission was removed.  But as noted
in the potential feature list up top, we'll continue to periodically revisit if there is a maintainable way of monitoring permission changes
in real time.

Conversely, if the kubelet is still in a retry cycle trying to launch a Pod with a `Share` reference, if now resolved permission issues were the only thing preventing 
a mount, the mount should then succeed.  Of course, as kubelet retry vs. controller re-list is the polling mechanism, and it is more frequent, the change in results would be more immediate in this case.


