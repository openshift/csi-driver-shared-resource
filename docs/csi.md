# Current status with respect to the Kubernetes CSIVolumeSource API

So let's take each part of the [CSIVolumeSource](https://github.com/kubernetes/api/blob/71efbb18d63cd30604981514ac623a6be1d413bb/core/v1/types.go#L1743-L1771):

- for the `Driver` string field, it needs to be ["csi-driver-projected-resource.openshift.io"](https://github.com/openshift/csi-driver-projected-resource/blob/1fcc354faa31f624086265ea2228661a0fc2e7b1/pkg/client/client.go#L28).
- for the `VolumeAttributes` map, this driver currently adds the "share" key (which maps the the `Share` instance your `Pod` wants to use) in addition to the
  elements of the `Pod` the kubelet stores when contacting the driver to provision the `Volume`.  See [this list](https://github.com/openshift/csi-driver-projected-resource/blob/c3f1c454f92203f4b406dabe8dd460782cac1d03/pkg/hostpath/nodeserver.go#L37-L42).
- the `ReadOnly` field is ignored, as the this driver's controller actively updates the `Volume` as the underlying `Secret` or `ConfigMap` change, or as
  the `Share` or the RBAC related to the `Share` change.  **NOTE:** we are looking at providing `ReadOnly` volume support in future updates.
- the `FSType` field is ignored.  This driver by design only supports `tmpfs`, with a different mount performed for each `Volume`, in order to defer all SELinux concerns to the kubelet.
- the `NodePublishSecretRef` field is ignored.  The CSI `NodePublishVolume` and `NodeUnpublishVolume` flows gate the permission evaluation required for the `Volume`
  by performing `SubjectAccessReviews` against the reference `Share` instance, using the `serviceAccount` of the `Pod` as the subject.
