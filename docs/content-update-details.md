# Details around pushing Secret and ConfigMap updates to provisioned Volumes

By default, the Shared Resource CSI Driver will watch for updates to `Secrets` and 
`ConfigMaps` in any of the namespaces referenced in corresponding `SharedSecrets` and 
`SharedConfigMaps`.  Then, when the actual `Secrets` or `ConfigMaps` referenced by those
`SharedSecrets` and `SharedConfigMaps` change, each volume across all the active `Pods`
in the system will have their content corresponding to those `Secrets` and `ConfigMaps`updated.

NOTE: this driver can still update the contents of the `Volumes` it provisions, even as it 
requires `Pods` to mark the `Volumes` as read-only.

This default behavior can be disabled at both [global level](config.md) and a per volume level.

The global setting takes precedence.  It is considered the domain of the cluster administrator,
with the belief that if they want resource refreshing disabled, it should be disabled for everyone.

For disabling at the volume specific level, the `volumeAttributes` field should have an entry with the 
key `refreshResource` and a value of `false`.

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: my-csi-app
  namespace: my-csi-app-namespace
spec:
  serviceAccountName: default
  containers:
  
  # specific container content would be here
  
  
  volumes:
    - name: my-csi-volume
      csi:
        readOnly: true
        driver: csi.sharedresource.openshift.io
        volumeAttributes:
          sharedConfigMap: my-share
          refreshResource: false
```

## NOTES

1) If the inverse on order of precedence gains favor as users start using this driver in earnest, we'll 
look into ways of providing that pattern.

2) This repository's maintainers are well aware of the "atomic writer" concept in upstream Kubernetes for coordinating
a Pod's access to secret volumes with the system's attempt to update them.  Such support is not yet integrated with
this driver, but we have plans to do so in a future release.
