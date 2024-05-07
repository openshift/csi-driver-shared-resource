# Local Development

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
