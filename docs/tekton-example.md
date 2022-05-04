# Tekton Example

As noted in this repositories [main README](../README.md#getting-started), Shared Resources are currently only available
on 'TechPreviewNoUpgrade' clusters.  So, unless you are employing [local development](../README.md#local-development)
you can convert your cluster to Tech Preview via:

```shell
kubectl patch featuregate cluster --type='merge' -p '{"spec":{"featureSet":"TechPreviewNoUpgrade"}}' 
```

Then, from the root directory, create/apply the various API objects defined in the YAML files in the `./examples` directory.
There are two examples there, where both use [the same namespace](../examples/00-namespace.yaml).

This document describes the [Tekton example](../examples/tekton).

For help on how to install the OpenShift Tekton offering, OpenShift Pipelies, look [here](https://docs.openshift.com/container-platform/4.10/cicd/pipelines/installing-pipelines.html).

This example includes `Task` and `TaskRun` [Tekton](https://github.com/tektoncd/pipeline) instances to leverage a mounted `SharedSecret`.
There are also various RBAC definitions that allow both
- the mounting of the `SharedSecret` specifically
- the use of `Volumes` of the `csi` type, if some form of Pod security is active on your cluster (since the Tekton controller
is the 'user' that creates the `Pod` from the `TaskRun`, we have to add permissions to use `csi` typed `Volumes` to the 
`ServiceAccount` of the `TaskRun`)
 
To run this example, execute:

```shell
$ kubectl apply -R -f ./examples
```

NOTE: if you are running against a cluster with some form of `Pod` security, and you as a user do not have permissions to 
use `Volumes` of type `csi`, nor does the `ServiceAccount` of the `Pod` (`default` with our example at this time), 
the creation of the test `Pod` will be rejected.

Ensure the `my-csi-app` Pod comes up in `Running` state.

Then, if you want to validate the volume, inspect the application pod `my-csi-app-pod`.

To verify, go back into the `TaskRun` named `my-csi-app-pod` and check the logs with the Tekton CLI `tkn`:

```shell
$ tkn taskrun logs -f my-taskrun-volume
````

You should see output like:

```shell
[list] total 0
[list] drwxrwxrwt. 3 root root 120 Apr 29 17:57 .
[list] dr-xr-xr-x. 1 root root  99 Apr 29 17:57 ..
[list] drwxr-xr-x. 2 root root  80 Apr 29 17:57 ..2022_04_29_17_57_36.1098027060
[list] lrwxrwxrwx. 1 root root  32 Apr 29 17:57 ..data -> ..2022_04_29_17_57_36.1098027060
[list] lrwxrwxrwx. 1 root root  11 Apr 29 17:57 key1 -> ..data/key1
[list] lrwxrwxrwx. 1 root root  11 Apr 29 17:57 key2 -> ..data/key2

[show-cannot-update] /tekton/scripts/script-1-lngp7: line 3: /data/foo: Read-only file system
```

How `Secrets` and `ConfigMaps` are stored on disk mirror the storage for
`Secrets` and `ConfigMaps` as done in the code in  [https://github.com/kubernetes/kubernetes](https://github.com/kubernetes/kubernetes)
where a file is created for each key in a `ConfigMap` `data` map or `binaryData` map and each key in a `Secret`
`data` map.