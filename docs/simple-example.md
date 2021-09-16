# Simple Example

From the root directory, deploy from the `./examples` directory the
application `Pod`, along with the associated test namespace, `SharedResource`, `ClusterRole`, and `ClusterRoleBinding` definitions
needed to illustrate the mounting of one of the API types (in this instance a `ConfigMap` from the `openshift-config`
namespace) into the `Pod`

For references, some in-lined yaml as a starting point.  Feel free to adjust the details to a working alternative that
meets your needs:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    openshift.io/cluster-monitoring: "true"
  name: my-csi-app-namespace
  
---

apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: shared-resource-my-share
  namespace: my-csi-app-namespace
rules:
  - apiGroups:
      - storage.openshift.io
    resources:
      - sharedresources
    resourceNames:
      - my-share
    verbs:
      - get
      - list
      - watch
      - use

---

apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: shared-resource-my-share
  namespace: my-csi-app-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: shared-resource-my-share
subjects:
  - kind: ServiceAccount
    name: default
    namespace: my-csi-app-namespace

---

apiVersion: storage.openshift.io/v1alpha1
kind: SharedResource
metadata:
  name: my-share
spec:
  backingResource:
    kind: ConfigMap
    apiVersion: v1
    name: openshift-install
    namespace: openshift-config

---

kind: Pod
apiVersion: v1
metadata:
  name: my-csi-app
  namespace: my-csi-app-namespace
spec:
  serviceAccountName: default
  containers:
    - name: my-frontend
      image: quay.io/quay/busybox
      volumeMounts:
        - mountPath: "/data"
          name: my-csi-volume
      command: [ "sleep", "1000000" ]
  volumes:
    - name: my-csi-volume
      csi:
        driver: csi-driver-shared-resource.openshift.io
        volumeAttributes:
          share: my-share

```

If you are fine with running this example as is, then execute:

```shell
$ kubectl apply -f ./examples
namespace/my-csi-app-namespace created
role.rbac.authorization.k8s.io/shared-resource-my-share created
rolebinding.rbac.authorization.k8s.io/shared-resource-my-share created
share.sharedresource.storage.openshift.io/my-share created
pod/my-csi-app created
```

Ensure the `my-csi-app` comes up in `Running` state.

Then, if you want to validate the volume, inspect the application pod `my-csi-app`.

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
