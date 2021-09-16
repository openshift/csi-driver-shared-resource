# OpenShift Shared Resource CSI Driver

The OpenShift Sharead Resource CSI Driver allows for the controlled (via Kubernetes RBAC) sharing of Kubernetes Secrets and ConfigMaps across 
Namespaces in Openshift.

The API used to achieve this support are:

- the `volume` and `volumeMount` fields of a Kubernetes Pod
- a new `Share` Kubernetes Custom Resource Definition which specifies which Secret of ConfigMap is to be shared, and which
serves as the resource in Kubernetes Subject Access Review checks 
  
## Features

- Supports only a subset of the Kubernetes CSIVolumeSource API.  See [CSI Volume Specifics](docs/csi.md) for details.
- Initial pod requests for `Share` CSI volumes are denied without both a valid `Share` reference and
  permissions to access that `Share`.
- Changes to the `Share`'s backing resource (kind, namespace, name) get reflected in data stored in the user pod's CSI volume.
- Subsequent removal of permissions for a `Share` results in removal of the associated data stored in the user pod's CSI volume.
- Re-granting of permission for a `Share` (after having the permissions initially, then removed) results in the associated
  data getting stored in the user pod's CSI volume.
- Removal of the `Share` used to provision a `Share` csi volume for a pod results in the associated data getting removed.  
- Re-creation of a removed `Share` for a previously provisioned `Share` CSI volume results in the associated data
  reappearing in the user pod's CSI volume.
- Supports recycling of the csi driver so that previously provisioned CSI volumes are still managed; in other words,
  the driver's internal state is persisted.
- Multiple `Shares` within a pod are allowed.
- When multiple `Shares` are mounted in a pod, one `Share` can be mounted as a subdirectory of another `Share`.


NOTE: see [CSI Volume Specifics](docs/csi.md) for restrictions around these features for read-only Volumes.

## Getting Started

Check out the [current installation options](docs/install.md) to get the driver up and going.  You'll need to have
sufficient privileges to create namespaces and ServiceAccounts, and then create `ClusterRoles`, `ClusterRoleBindings`, `DaemonSets` with the privileged bit set,
and the creation of `CSIDrivers`.

Then, check out our [entry level example](docs/simple-example.md).  You'll need to have sufficient privileges to create
namespaces, `Roles` and `RoleBindings`, instances of our new `Share` CRD, and pods.

The permission semantics in summary:
- the `ServiceAccount` associated with a `Pod` needs access to the 'use' verb on the `Share` referenced any `CSIVolume`
specified in a `Pod` that uses this repository's CSI Driver.
- separately, any `User` can discover cluster scoped `Shares` based on the permissions granted to them by their cluster
or namespace administrator.

The full definition of the `Share` custom resource can be found [here](deploy/0000_10_projectedresource.crd.yaml).

For a more real world example of using this new driver to help with sharing RHEL entitlements, [this blog post](https://www.openshift.com/blog/the-path-to-improving-the-experience-with-rhel-entitlements-on-openshift)
dives into that scenario.

Next, for some details around support for updating `Share` volumes as their corresponding Secrets and ConfigMaps change,
please visit [here](docs/content-update-details.md).

Lastly, for a depiction of details around the [features noted above](#features), check out this [FAQ](docs/faq.md).
