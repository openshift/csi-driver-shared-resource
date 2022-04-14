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

Then, if you want to validate the volume, inspect the application pod `my-csi-app-pod`.

To verify, go back into the `Pod` named `my-csi-app-pod` and list the contents:

  ```shell
  $ kubectl exec  -n my-csi-app-namespace -it my-csi-app-pod /bin/sh
  / # ls -lR /data
  ```

You should see contents like:

```shell
/ # ls -lR /data 
ls -lR /data 
/data:
total 0
lrwxrwxrwx    1 root     root            11 Apr 14 14:37 key1 -> ..data/key1
lrwxrwxrwx    1 root     root            11 Apr 14 14:37 key2 -> ..data/key2
/ # 
```

**NOTE**: You'll notice that the driver has created subdirectories off of the `volumeMount` specified in our example `Pod`.
One subdirectory for the type (`configsmaps` or `secrets`), and one whose name is a concatenation of the `namespace` and
`name` of the `ConfigMap` or `Secret` being mounted.  As noted in the high level feature list above, new features that allow
some control on how the files are laid out should be coming.

Now, if you inspect the contents of that `ConfigMap`, you'll see keys in the `data` map that
correspond to the 2 files created:

```shell
$ oc get cm my-config -o yaml
apiVersion: v1
data:
  key1: config1
  key2: config2
kind: ConfigMap
metadata:
  creationTimestamp: "2022-04-14T14:01:13Z"
  name: my-config
  namespace: my-csi-app-namespace
  resourceVersion: "35201"
  uid: fec12a32-79fc-44ef-b1dd-965b72041bbe
```

How `Secrets` and `ConfigMaps` are stored on disk mirror the storage for
`Secrets` and `ConfigMaps` as done in the code in  [https://github.com/kubernetes/kubernetes](https://github.com/kubernetes/kubernetes)
where a file is created for each key in a `ConfigMap` `data` map or `binaryData` map and each key in a `Secret`
`data` map.