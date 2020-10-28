# OpenShift Projected Resource "CSI DRIVER"

The Work In Progress implementation for [this OpenShift Enhancement Proposal](https://github.com/openshift/enhancements/blob/master/enhancements/cluster-scope-secret-volumes/csi-driver-host-injections.md),
this repository borrows from this reference implementation:

- [CSI Hostpath Driver](https://github.com/kubernetes-csi/csi-driver-host-path)
- [Node Driver Registrar Sidecar Container](https://github.com/kubernetes-csi/node-driver-registrar)

As part of forking these into this repository, function not required for the projected resources scenarios have 
been removed, and the images containing these commands are built off of RHEL based images instead of the non-Red Hat
images used upstream.

And will in the near future cherry pick overlapping CSI API implementation logic from:

- [Kubernetes-Secrets-Store-CSI-Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver)

As the enhancement proposal reveals, this is not a fully compliant CSI Driver implementation.  This repository
solely provides the minimum amounts of the Kubernetes / CSI contract to achive the goals stipulated in the 
Enhancement proposal.

## Current status with respect to the enhancement propsal

**NOT FULLY IMPLEMENTED**

The latest commit of the master branch solely introduces both the `Share` CRD and the `projectedresoure.storage.openshift.io`
API group and version `v1alpha1`.  

The reference to the `share` object in the `volumeAttributes` in a declared CSI volume within a `Pod` is used to 
fuel a `SubjectAccessReview` check.  The `ServiceAccount` for the `Pod` must have `get` access to the `Share` in
order for the referenced `ConfigMap` and `Secret` to be mounted in the `Pod`.

A controller exists for watching this new CRD, as well as `ConfigMaps` and `Secrets` in all Namespaces except for
a list of OpenShift "system" namespaces which have `ConfigMaps` that get updated every few seconds.
 
The controller and CSI driver in their current form facilitate the following scenarios:

- initial pod requests for share csi volumes are denied without both a valid share refrence and 
permissions to access that share
- changes to the share's backing resource (kind, namespace, name) get reflected in data stored in the user pod's CSI volume
- subsequent removal of permissions for a share results in removal of the associated data stored in the user pod's CSI volume
- re-granting of permission for a share (after having the permissions initially, then removed) results in the associated 
data getting stored in the user pod's CSI volume
- removal of the share used to provision share csi volume for a pod result in the associated data getting removed
- re-creation of a removed share for a previously provisioned share CSI volume results in the associated data 
reappearing in the user pod's CSI volume
- support recycling of the csi driver so that previously provisioned CSI volumes are still managed; in other words,
the driver's interan state is persisted 

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

## Deployment
The easiest way to test the Hostpath driver is to run the `deploy.sh`.

```
# deploy csi projectedresource driver
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
$ kubectl describe pods/my-csi-app
Name:         my-csi-app
Namespace:    csi-driver-projected-resource
Priority:     0
Node:         ip-10-0-163-121.us-west-2.compute.internal/10.0.163.121
Start Time:   Wed, 05 Aug 2020 14:23:57 -0400
Labels:       <none>
Annotations:  k8s.v1.cni.cncf.io/network-status:
                [{
                    "name": "",
                    "interface": "eth0",
                    "ips": [
                        "10.129.2.16"
                    ],
                    "default": true,
                    "dns": {}
                }]
              k8s.v1.cni.cncf.io/networks-status:
                [{
                    "name": "",
                    "interface": "eth0",
                    "ips": [
                        "10.129.2.16"
                    ],
                    "default": true,
                    "dns": {}
                }]
              openshift.io/scc: node-exporter
Status:       Running
IP:           10.129.2.16
IPs:
  IP:  10.129.2.16
Containers:
  my-frontend:
    Container ID:  cri-o://cf4cd4f202d406153e3a067f6f6926ae93dd9748923a5116b2e2ee27e00d33e6
    Image:         busybox
    Image ID:      docker.io/library/busybox@sha256:400ee2ed939df769d4681023810d2e4fb9479b8401d97003c710d0e20f7c49c6
    Port:          <none>
    Host Port:     <none>
    Command:
      sleep
      1000000
    State:          Running
      Started:      Wed, 05 Aug 2020 14:24:03 -0400
    Ready:          True
    Restart Count:  0
    Environment:    <none>
    Mounts:
      /data from my-csi-volume (rw)
      /var/run/secrets/kubernetes.io/serviceaccount from default-token-xbsjd (ro)
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
    VolumeAttributes:  <none>
  default-token-xbsjd:
    Type:        Secret (a volume populated by a Secret)
    SecretName:  default-token-xbsjd
    Optional:    false
QoS Class:       BestEffort
Node-Selectors:  <none>
Tolerations:     node.kubernetes.io/not-ready:NoExecute for 300s
                 node.kubernetes.io/unreachable:NoExecute for 300s
Events:
  Type    Reason          Age        From                                                 Message
  ----    ------          ----       ----                                                 -------
  Normal  Scheduled       <unknown>                                                       Successfully assigned csi-driver-projected-resource/my-csi-app to ip-10-0-163-121.us-west-2.compute.internal
  Normal  AddedInterface  28m        multus                                               Add eth0 [10.129.2.16/23]
  Normal  Pulling         28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Pulling image "busybox"
  Normal  Pulled          28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Successfully pulled image "busybox" in 3.626604306s
  Normal  Created         28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Created container my-frontend
  Normal  Started         28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Started container my-frontend
```


## Confirm openshift-config ConfigMap data is present

This current version of the driver as POC also watches the `ConfigMaps` and `Secrets` in the `openshift-config` 
namespace and places that data in the provide `Volume` as well.

To verify, go back into the `Pod` named `my-csi-app` and list the contents:

  ```shell
  $ kubectl exec  -n my-csi-app-namespace -it my-csi-app /bin/sh
  / # ls -lR /data
  / # exit
  ```

You should see contents like:

```shell
/ # ls -lR /data 
ls -lR /data 
/data:
total 0
drwxr-xr-x    3 root     root            60 Oct 28 14:52 configmaps

/data/configmaps:
total 0
drwxr-xr-x    2 root     root            80 Oct 28 14:52 openshift-config:openshift-install

/data/configmaps/openshift-config:openshift-install:
total 8
-rw-r--r--    1 root     root             4 Oct 28 14:52 invoker
-rw-r--r--    1 root     root            70 Oct 28 14:52 version
/ # 
```

And if you inspect the contents of that `ConfigMap`, you'll see keys in the `data` map that 
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

The storage of how `Secrets` and `ConfigMaps` are stored on disk mirror the corresponding volume types for 
`Secrets` and `ConfigMaps` are stored per the code in  [https://github.com/kubernetes/kubernetes](https://github.com/kubernetes/kubernetes)
where a file is created for each key in a `ConfigMap` `data` map or `binaryData` map and each key in a `Secret`
`data` map.

If you want to try other `ConfigMaps` or a `Secret`, first clear out the existing application:

```shell
$ oc delete -f ./examples 
``` 

And the edit `./examples/02-csi-share.yaml` and change the `backingResource` stanza to point to the item 
you want to share, and then re-run `oc apply -f ./examples`.
