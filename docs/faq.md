# Frequently Asked Questions

## What happens if the SharedConfigMap or SharedSecret does not exist when you create a Pod that references it?

You'll see an event like:

```bash
$ oc get events
0s          Warning   FailedMount      pod/my-csi-app                                       MountVolume.SetUp failed for volume "my-csi-volume" : rpc error: code = InvalidArgument desc = the csi driver volumeAttribute 'share' reference had an error: sharedconfigmap.sharedresource.openshift.io "my-share" not found
$
```

And your Pod will never reach the Running state.

However, if the kubelet is still in a retry cycle trying to launch a Pod with a `SharedConfigMap` or `SharedSecret` reference, if `SharedConfigMap` or `SharedSecret` non-existence is the only thing preventing a mount, the mount should then succeed if the `SharedConfigMap` or `SharedSecret` comes into existence.

## What happens if the SharedConfigMap or SharedSecret is removed after the pod starts?

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

## What happens if the Role or RoleBinding are not present when your newly created Pod tries to access an existing SharedConfigMap or SharedSecret?

```bash
$ oc get events
LAST SEEN   TYPE      REASON        OBJECT           MESSAGE
6s          Normal    Scheduled     pod/my-csi-app   Successfully assigned my-csi-app-namespace/my-csi-app to ip-10-0-136-162.us-west-2.compute.internal
2s          Warning   FailedMount   pod/my-csi-app   MountVolume.SetUp failed for volume "my-csi-volume" : rpc error: code = PermissionDenied desc = subjectaccessreviews sharedresource my-share podNamespace my-csi-app-namespace podName my-csi-app podSA default returned forbidden
$

```
And your Pod will never get to the Running state.

## What happens if the Pod successfully mounts a SharedConfigMap or SharedSecret, and later the permissions to access the SharedConfigMap or SharedSecret are removed?

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

Do note that if your Pod copied the data to other locations, the Shared Resource driver cannot do anything about those copies.  A big motivator for allowing
some customization of the directory and file structure off of the `volumeMount` of the `Pod` is to help reduce the *need* to copy
files.  Hopefully you can mount that data directly at its final, needed, destination.

Also note that the Shared Resource driver does not try to reverse engineer which RoleBinding or ClusterRoleBinding allows your Pod to access the `SharedConfigMap` or `SharedSecret`.
The Kubernetes and OpenShift libraries for this are not currently structured to be openly consumed by other components.  Nor did we entertain taking
snapshots of that code to serve such a purpose.  So instead of listening to RoleBinding or Role changes, on the Shared Resource controller’s re-list interval
(which is configurable via start up argument on the command invoked from out DaemonSet, and whose default is 10 minutes), the controller will re-execute
Subject Access Review requests for each Pod’s reference to each `SharedConfigMap` or `SharedSecret` on the `SharedConfigMap` or `SharedSecret` re-list and remove content if permission was removed.  But as noted
in the potential feature list up top, we'll continue to periodically revisit if there is a maintainable way of monitoring permission changes
in real time.

Conversely, if the kubelet is still in a retry cycle trying to launch a Pod with a `SharedConfigMap` or `SharedSecret` reference, if now resolved permission issues were the only thing preventing
a mount, the mount should then succeed.  Of course, as kubelet retry vs. controller re-list is the polling mechanism, and it is more frequent, the change in results would be more immediate in this case.

# Other Secret Providers/Operators

The Shared Resource CSI driver has similar features and technical capabilities as
other "secrets operator" projects. This CSI driver is complementary to many of these
projects, especially those designed to fetch secrets from external providers such as
HashiCorp Vault, AWS, Azure, and other cloud provider secret storage.

## Can't I use the External Secrets Operator to share resources in my cluster?

For Secrets, yes. The [External Secrets Operator](https://external-secrets.io) can
share Secrets through its [Kubernetes provider](https://external-secrets.io/latest/provider/kubernetes/).
However, there are several technical differences between this and External Secrets
operator - see the "Feature Comparison" section below.

## Can I share resources with the Secret Store CSI Driver?

Not at present. Like this project, the [Secret Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io)
allows secrets to be synced and mounted with `CSI` volumes. A separate [provider](https://secrets-store-csi-driver.sigs.k8s.io/providers)
is responsible for fetching and syncing the referenced data. As of this writing, no
provider syncs data that explicitly exists within a Kubernetes cluster.

Early in this project's history, we considered implementing a provider for the
Secret Store CSI Driver. This was declined in favor of the current RBAC model and
implementing the CSI interfaces required for `Ephemeral` volume lifecycle mode.

## Can I deploy the Shared Resource CSI Driver and Secret Store CSI Driver on the same cluster?

Yes. The Shared Resource CSI Driver can be deployed side by side with any other CSI
driver, or other tools that sync external data onto a cluster.

## Feature Comparison

| Feature | Shared Resources | External Secrets | Secret Store CSI |
| ------- | ---------------- | ---------------- | ---------------- |
| Share Secrets | ✅  | ✅ | ✅ External |
| Share ConfigMaps | ✅ | ❌ | ❌ |
| Share in same Kubernetes Cluster | ✅ | ✅ | ❌ External |
| Share across Kubernetes Clusters | ❌ | ✅ | ✅ External |
| Sync data | ✅ Resource watching | ✅ Polling | ✅ Alpha feature |
| Mount with "mirror" `Secret` object | ❌ | ✅ | ✅ Optional |
| Mount in pods with CSI volume | ✅ | ❌ | ✅ |
| Share subset of keys in `Secret` | ❌ | ✅ | ✅ Provider specific |
| Control access to `Secret` data with RBAC by Namespace | ✅ | ✅ | ✅ |
| Control access to `Secret` data with RBAC by User/ServiceAccount | ✅ | ❌ | ❌ |
