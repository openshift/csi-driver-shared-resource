# Simple Example

From the root directory, create/apply the various API objects defined in the YAML files in the `./examples` directory.
There are two examples there, where both use [the same namespace](../examples/00-namespace.yaml).

This document describes the [simple Pod example](../examples/simple).

The example is an application `Pod`, along with `SharedConfigMap`, `Role`, and `RoleBinding` definitions
needed to illustrate the mounting of one of the API types (in this instance a `ConfigMap` from the `openshift-config`
namespace) into the `Pod`.
 
To run this example, execute:

```shell
$ kubectl apply -R -f ./examples
```

Ensure the `my-csi-app` Pod comes up in `Running` state.

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