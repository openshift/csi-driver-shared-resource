# Current status with respect to the Kubernetes CSIVolumeSource API

So let's take each part of the [CSIVolumeSource](https://github.com/kubernetes/api/blob/71efbb18d63cd30604981514ac623a6be1d413bb/core/v1/types.go#L1743-L1771):

- for the `Driver` string field, it needs to be "csi.sharedresource.openshift.io".
- for the `VolumeAttributes` map, this driver currently inspects the "sharedConfigMap" key or "sharedSecret" key (which map the `SharedConfigMap` OR `SharedSecret` instance your `Pod` wants to use) in addition to the
  elements of the `Pod` the kubelet stores when contacting the driver to provision the `Volume`.  See [this list](https://github.com/openshift/csi-driver-shared-resource/blob/c3f1c454f92203f4b406dabe8dd460782cac1d03/pkg/hostpath/nodeserver.go#L37-L42).
- NOTE: you cannot specify both a "sharedConfigMap" and "sharedSecret" key.  An error will be flagged.  An error will also be flagged if neither is present, or if the value for one or the other does not equal the name of a `SharedConfigMap` or `SharedSecret`
- the `ReadOnly` field is honored.  The driver can even update the content for a read-only `Volume`, even as the `Pod` consuming the `Volume` cannot write to the `Volume`.  However,
  a limitation exists which makes any updates of content for read-only `Volumes` *OFFICIALLY* unsupported.  Namely, the function will not work across driver restarts.  The driver
  loses access to the kubelet's mount for the `Volume`.  This is not the case for read-write `Volumes`.  The driver is able to update the content of read-write `Volumes` provisioned before the 
  latest restart of the driver.
- Also, mounting of one `SharedConfigMap` OR `SharedSecret` off of a subdirectory of another `SharedConfigMap` OR `SharedSecret` is only supported with read-write `Volumes`.  
- the `FSType` field is ignored.  This driver by design only supports `tmpfs`, with a different mount performed for each `Volume`, in order to defer all SELinux concerns to the kubelet.
- the `NodePublishSecretRef` field is ignored.  The CSI `NodePublishVolume` and `NodeUnpublishVolume` flows gate the permission evaluation required for the `Volume`
  by performing `SubjectAccessReviews` against the reference `SharedConfigMap` OR `SharedSecret` instance, using the `serviceAccount` of the `Pod` as the subject.
