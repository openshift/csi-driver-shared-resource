# Current status with respect to the Kubernetes CSIVolumeSource API

So let's take each part of the [CSIVolumeSource](https://github.com/kubernetes/api/blob/71efbb18d63cd30604981514ac623a6be1d413bb/core/v1/types.go#L1743-L1771):

- for the `Driver` string field, it needs to be "csi.sharedresource.openshift.io".
- for the `VolumeAttributes` map, this driver currently inspects the "sharedConfigMap" key or "sharedSecret" key (which map the `SharedConfigMap` OR `SharedSecret` instance your `Pod` wants to use) in addition to the
  elements of the `Pod` the kubelet stores when contacting the driver to provision the `Volume`.  See [this list](https://github.com/openshift/csi-driver-shared-resource/blob/c3f1c454f92203f4b406dabe8dd460782cac1d03/pkg/hostpath/nodeserver.go#L37-L42).
- NOTE: you cannot specify both a "sharedConfigMap" and "sharedSecret" key.  An error will be flagged.  An error will also be flagged if neither is present, or if the value for one or the other does not equal the name of a `SharedConfigMap` or `SharedSecret`
- the `ReadOnly` field is required to be set to 'true'.  This follows conventions introduced in upstream Kubernetes CSI Drivers to facilitate proper SELinux labelling.  What occurs is that
this driver will return a read-write linux file system to the kubelet, so that CRI-O can apply the correct SELinux labels on the file system (CRI-O would not be able to update the SELinux labels on a read only file system
after it is created), but the kubelet still makes sure that the file system later exposed to the consuming pod (which sits on top of the file system returned by this repository's driver) is read only.
If this driver allowed both read-only and read-write, there is in fact no way to provide differing support that still allows for correct SELinux labelling for each).
- Also, mounting of one `SharedConfigMap` OR `SharedSecret` off of a subdirectory of another `SharedConfigMap` OR `SharedSecret` is *NOT* supported. The driver only supports read-only `Volumes`.  
- the `FSType` field is ignored.  This driver by design only supports `tmpfs`, with a different mount performed for each `Volume`, in order to defer all SELinux concerns to the kubelet.
- the `NodePublishSecretRef` field is ignored.  The CSI `NodePublishVolume` and `NodeUnpublishVolume` flows gate the permission evaluation required for the `Volume`
  by performing `SubjectAccessReviews` against the reference `SharedConfigMap` OR `SharedSecret` instance, using the `serviceAccount` of the `Pod` as the subject.
- Similar to what is noted for the upstream "Secrets Store CSI Driver", because of the use of atomic writer, neither `Secret` or `ConfigMap` content is rotated when using 'subPath' volume mounts.
  The upstream driver documentation has a good explanation as to the details [here](https://secrets-store-csi-driver.sigs.k8s.io/known-limitations.html#secrets-not-rotated-when-using-subpath-volume-mount)
