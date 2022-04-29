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

The maintenance of the related API objects and the deployment of this CSI driver are handled via the [Openshift CSI Driver for Shared Resources Operator](https://github.com/openshift/csi-driver-shared-resource-operator)
when you are using a Tech Preview OpenShift Cluster in 4.10.  The 4.10 release docs are [here](https://docs.openshift.com/container-platform/4.10/storage/container_storage_interface/ephemeral-storage-shared-resource-csi-driver-operator.html),
and these [4.10 docs](https://docs.openshift.com/container-platform/4.10/post_installation_configuration/cluster-tasks.html#post-install-tp-tasks) are 
sufficient for explaining how to turn on Tech Preview features after install.

For running on a 4.10 cluster which is *NOT* a Tech Preview cluster, you must employ the methodology described in the [Local Devlopment](#local-development)
section.

Once installed, the permission semantics around sharing resources is the next concern to consider.  In summary:
- the `ServiceAccount` associated with a `Pod` needs access to the 'use' verb on the `SharedConfigMap` or `SharedSecret` referenced by any `CSIVolume`
specified in a `Pod` that uses this repository's CSI Driver.
- separately, any `User` can discover cluster scoped `SharedResources` based on the 'get' or 'list' permissions granted to them by their cluster
or namespace administrator.

The full definition of the `SharedConfigMap` can be found [here](deploy/0000_10_sharedconfigmap.crd.yaml) or `SharedSecret` custom resource can be found [here](deploy/0000_10_sharedsecret.crd.yaml).

Under the [examples directory](examples) there is both a simple example of using a `SharedConfigMap` in a `Pod`, and an
example of performing an OpenShift `Build` from a `BuildConfig` that uses Red Hat Entitlements available from your Red Hat Subscription when
your subscription credentials are stored on your cluster via OpenShift Insights Operator, and resulting `Secret` for those
credentials is made available to additional `Namespaces` via a `SharedSecret`.  Instructions for the `Pod` example are
[here](docs/simple-example.md), and for the `BuildConfig` example are [here](docs/build-with-rhel-entitlements.md).

Next, for some details around support for updating `SharedConfigMap` or `SharedSecret` volumes as their corresponding Secrets or ConfigMaps change,
please visit [here](docs/content-update-details.md).

Lastly, for a depiction of details around the [features noted above](#features), check out this [FAQ](docs/faq.md).

## Local Development

If you are going to make code changes to this driver, and you'd like to test them against an OpenShift cluster, run the 
`build-image` make target in this repository to capture those changes in an image reference whose remote registry and repository you can push
to, and then employ the steps described in the [Openshift CSI Driver for Shared Resources Operator Quick Start](https://github.com/openshift/csi-driver-shared-resource-operator/blob/master/README.md#quick-start),
where you set the `DRIVER_IMAGE` environment variable to the image reference created by your `make build-image` against
your local clone of this repository.

See that operator's [quick start guide](https://github.com/openshift/csi-driver-shared-resource-operator#quick-start) for 
complete details.

NOTE: changes to API objects that act in concert with the driver (RBAC, CSI Driver definition, service, serviceaccounts, etc)
are defined at [https://github.com/openshift/csi-driver-shared-resource-operator/tree/master/assets](https://github.com/openshift/csi-driver-shared-resource-operator/tree/master/assets).
If your changes need adjustments to those objects, you'll need to use `make deploy` to rollout a new version of the operator,
per the same quick start guide.
