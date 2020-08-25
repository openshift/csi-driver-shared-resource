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

The latest commit of the master branch solely brings in the subset of the hostpath driver needed to validate 
the basic in-line ephemeral volume scenario articulated below. 


## Deployment
The easiest way to test the Hostpath driver is to run the `deploy.sh`.

```
# deploy hostpath driver
$ deploy/deploy.sh
```

You should see an output similar to the following printed on the terminal showing the application of rbac rules and the
result of deploying the hostpath driver, external provisioner, external attacher and snapshotter components. Note that the following output is from Kubernetes 1.17:

```shell
deploying hostpath components
   deploy/hostpath/00-namespace.yaml
kubectl apply -f deploy/hostpath/00-namespace.yaml
namespace/projected-resource-csi-driver created
   deploy/hostpath/01-service-account.yaml
kubectl apply -f deploy/hostpath/01-service-account.yaml
serviceaccount/projected-resource-csi-driver-plugin created
   deploy/hostpath/03-cluster-role-binding.yaml
kubectl apply -f deploy/hostpath/03-cluster-role-binding.yaml
clusterrolebinding.rbac.authorization.k8s.io/projected-resource-privileged created
   deploy/hostpath/csi-hostpath-driverinfo.yaml
kubectl apply -f deploy/hostpath/csi-hostpath-driverinfo.yaml
csidriver.storage.k8s.io/projected-resource-csi-driver.openshift.io created
   deploy/hostpath/csi-hostpath-plugin.yaml
kubectl apply -f deploy/hostpath/csi-hostpath-plugin.yaml
service/csi-hostpathplugin created
daemonset.apps/csi-hostpathplugin created
```

## Run example application and validate

First, let's validate the deployment.  Ensure all expected pods are running for the driver plugin, which in a 
3 node OCP cluster will look something like:

```shell
$ kubectl get pods -n projected-resource-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
csi-hostpathplugin-c7bbk   2/2     Running   0          23m
csi-hostpathplugin-m4smv   2/2     Running   0          23m
csi-hostpathplugin-x9xjw   2/2     Running   0          23m
```

Next, let's start up the simple test application.  From the root directory, deploy the application pod found in directory `./examples`:

```shell
$ kubectl apply -f ./examples/csi-app.yaml
pod/my-csi-app created
```

Ensure the `my-csi-app` comes up in `Running` state.

Finally, if you want to validate the volume, inspect the application pod `my-csi-app`:

```shell
$ kubectl describe pods/my-csi-app
Name:         my-csi-app
Namespace:    projected-resource-csi-driver
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
    Driver:            projected-resource-csi-driver.openshift.io
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
  Normal  Scheduled       <unknown>                                                       Successfully assigned projected-resource-csi-driver/my-csi-app to ip-10-0-163-121.us-west-2.compute.internal
  Normal  AddedInterface  28m        multus                                               Add eth0 [10.129.2.16/23]
  Normal  Pulling         28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Pulling image "busybox"
  Normal  Pulled          28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Successfully pulled image "busybox" in 3.626604306s
  Normal  Created         28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Created container my-frontend
  Normal  Started         28m        kubelet, ip-10-0-163-121.us-west-2.compute.internal  Started container my-frontend
```

## Confirm the driver works
The driver is configured to create new volumes under `/csi-data-dir` inside the hostpath container that is specified in 
the plugin DaemonSet. 

A file written in a properly mounted Hostpath volume inside an application should show up inside the Hostpath container.  
The following steps confirms that Hostpath is working properly.  First, create a file from the application pod as shown:

```shell
$ kubectl exec -it my-csi-app /bin/sh
/ # touch /data/hello-world
/ # exit
```

Next, ssh into the Hostpath containers of each of the 3 pods and verify that the file shows up.  

**NOTE**:  Given node affinity, the file will only show up in one of the 3 containers.  That's OK :-).  When we are 
done, we want our projected resource shared data access to always be local node.  Each of the pods will watch 
the projected resources and store on their local hosts.

So get the list of `csi-hostpathplugin*` pod names, and then: 

```shell
$ kubectl exec -it <each of the 3 pod names> -c hostpath /bin/sh

```
Then, use the following command to locate the file. If everything works OK you should get a result similar to the following:

```shell
/ # find / -name hello-world
/var/lib/kubelet/pods/34bbb561-d240-4483-a56c-efcc6504518c/volumes/kubernetes.io~csi/pvc-ad827273-8d08-430b-9d5a-e60e05a2bc3e/mount/hello-world
/csi-data-dir/42bdc1e0-624e-11ea-beee-42d40678b2d1/hello-world
/ # exit
```

## Confirm openshift-config Secret/ConfigMap data present

This current version of the driver as POC also watches the `ConfigMaps` and `Secrets` in the `openshift-config` 
namespace and places that data in the provide `Volume` as well.

To verify, go back into the `Pod` named `my-csi-app` and list the contents:

  ```shell
  $ kubectl exec -it my-csi-app /bin/sh
  / # ls -lR /data
  / # exit
  ```

You should see contents like:

```shell
ls -lR /data
/data:
total 8
drwxr-xr-x    2 root     root          4096 Aug  7 16:07 configmaps
drwxr-xr-x    2 root     root          4096 Aug  7 16:07 secrets

/data/configmaps:
total 36
-rw-r--r--    1 root     root          2050 Aug  7 16:07 openshift-config:admin-kubeconfig-client-ca
-rw-r--r--    1 root     root          1992 Aug  7 16:07 openshift-config:etcd-ca-bundle
-rw-r--r--    1 root     root          2024 Aug  7 16:07 openshift-config:etcd-metric-serving-ca
-rw-r--r--    1 root     root          1994 Aug  7 16:07 openshift-config:etcd-serving-ca
-rw-r--r--    1 root     root           840 Aug  7 16:07 openshift-config:initial-etcd-ca
-rw-r--r--    1 root     root          6848 Aug  7 16:07 openshift-config:initial-kube-apiserver-server-ca
-rw-r--r--    1 root     root           970 Aug  7 16:07 openshift-config:openshift-install
-rw-r--r--    1 root     root           990 Aug  7 16:07 openshift-config:openshift-install-manifests

/data/secrets:
total 236
-rw-r--r--    1 root     root         12749 Aug  7 16:07 openshift-config:builder-dockercfg-lnx4c
-rw-r--r--    1 root     root         24100 Aug  7 16:07 openshift-config:builder-token-r87zm
-rw-r--r--    1 root     root         23657 Aug  7 16:07 openshift-config:builder-token-s2rcl
-rw-r--r--    1 root     root         12749 Aug  7 16:07 openshift-config:default-dockercfg-pzxpf
-rw-r--r--    1 root     root         23657 Aug  7 16:07 openshift-config:default-token-fk5mn
-rw-r--r--    1 root     root         24100 Aug  7 16:07 openshift-config:default-token-pv2n2
-rw-r--r--    1 root     root         12806 Aug  7 16:07 openshift-config:deployer-dockercfg-rk4mr
-rw-r--r--    1 root     root         23668 Aug  7 16:07 openshift-config:deployer-token-ktqgk
-rw-r--r--    1 root     root         24111 Aug  7 16:07 openshift-config:deployer-token-mnglk
-rw-r--r--    1 root     root          4764 Aug  7 16:07 openshift-config:etcd-client
-rw-r--r--    1 root     root          4822 Aug  7 16:07 openshift-config:etcd-metric-client
-rw-r--r--    1 root     root          4730 Aug  7 16:07 openshift-config:etcd-metric-signer
-rw-r--r--    1 root     root          4696 Aug  7 16:07 openshift-config:etcd-signer
-rw-r--r--    1 root     root          3185 Aug  7 16:07 openshift-config:initial-service-account-private-key
-rw-r--r--    1 root     root          4729 Aug  7 16:07 openshift-config:pull-secret
```

To facilitate validation of the contents, including post-creation updates, the data is currently 
stored as formatted `json`. 
