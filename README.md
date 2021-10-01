# OpenShift Shared Resource CSI Driver

The OpenShift Shared Resource CSI Driver allows for the controlled (via Kubernetes RBAC) sharing of Kubernetes Secrets and ConfigMaps across 
Namespaces in Openshift.

The API used to achieve this support are:

- the `volume` and `volumeMount` fields of a Kubernetes Pod
- a new `SharedConfigMap` Kubernetes Custom Resource Definition which specifies which ConfigMap is to be shared, and which
serves as the resource in Kubernetes Subject Access Review checks 
- a new `SharedSecret` Kubernetes Custom Resource Definition which specifies which Secret is to be shared, and which
  serves as the resource in Kubernetes Subject Access Review checks
  
## Features

- Supports only a subset of the Kubernetes CSIVolumeSource API.  See [CSI Volume Specifics](docs/csi.md) for details.
- Initial pod requests for `SharedConfigMap` or `SharedSecret` CSI volumes are denied without both a valid `SharedConfigMap` or `SharedSecret` reference and
  permissions to access that `SharedConfigMap` or `SharedSecret`.
- Changes to the `SharedConfigMap` or `SharedSecret` backing resource (namespace, name) get reflected in data stored in the user pod's CSI volume.
- //TODO - do we now change the ability to change within a Pod between `SharedConfigMap` and `SharedSecret`
- Subsequent removal of permissions for a `SharedConfigMap` or `SharedSecret` results in removal of the associated data stored in the user pod's CSI volume.
- Re-granting of permission for a `SharedConfigMap` or `SharedSecret` (after having the permissions initially, then removed) results in the associated
  data getting stored in the user pod's CSI volume.
- Removal of the `SharedConfigMap` or `SharedSecret` used to provision a `SharedConfigMap` or `SharedSecret` csi volume for a pod results in the associated data getting removed.  
- Re-creation of a removed `SharedConfigMap` or `SharedSecret` for a previously provisioned `SharedConfigMap` or `SharedSecret` CSI volume results in the associated data
  reappearing in the user pod's CSI volume.
- Supports recycling of the csi driver so that previously provisioned CSI volumes are still managed; in other words,
  the driver's internal state is persisted.
- Multiple `SharedResources` within a pod are allowed.
- When multiple `SharedResources` are mounted in a pod, one `SharedConfigMap` or `SharedSecret` can be mounted as a subdirectory of another `SharedConfigMap` or `SharedSecret`.


NOTE: see [CSI Volume Specifics](docs/csi.md) for restrictions around these features for read-only Volumes.

## Getting Started

Check out the [current installation options](docs/install.md) to get the driver up and going.  You'll need to have
sufficient privileges to create namespaces and ServiceAccounts, and then create `ClusterRoles`, `ClusterRoleBindings`, `DaemonSets` with the privileged bit set,
and the creation of `CSIDrivers`.

Then, check out our [entry level example](docs/simple-example.md).  You'll need to have sufficient privileges to create
namespaces, `Roles` and `RoleBindings`, instances of our new `SharedConfigMap` or `SharedSecret` CRD, and pods.

The permission semantics in summary:
- the `ServiceAccount` associated with a `Pod` needs access to the 'use' verb on the `SharedConfigMap` or `SharedSecret` referenced any `CSIVolume`
specified in a `Pod` that uses this repository's CSI Driver.
- separately, any `User` can discover cluster scoped `SharedResources` based on the permissions granted to them by their cluster
or namespace administrator.

The full definition of the `SharedConfigMap` can be found [here](deploy/0000_10_sharedconfigmap.crd.yaml) or `SharedSecret` custom resource can be found [here](deploy/0000_10_sharedsecret.crd.yaml).

For a more real world example of using this new driver to help with sharing RHEL entitlements, [this blog post](https://www.openshift.com/blog/the-path-to-improving-the-experience-with-rhel-entitlements-on-openshift)
dives into that scenario.

Next, for some details around support for updating `SharedConfigMap` or `SharedSecret` volumes as their corresponding Secrets or ConfigMaps change,
please visit [here](docs/content-update-details.md).

Lastly, for a depiction of details around the [features noted above](#features), check out this [FAQ](docs/faq.md).
