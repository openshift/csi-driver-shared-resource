# RBAC Configuration for Shared Resources

The Shared Resource CSI Driver enforces two layers of RBAC before mounting a
`SharedSecret` or `SharedConfigMap` into a Pod:

1. **Driver-level RBAC** — The CSI driver's ServiceAccount must have
   `get`, `list`, and `watch` permissions on the source Secret or ConfigMap in
   its namespace.
2. **Pod-level RBAC** — The Pod's ServiceAccount must have `use` permission on
   the `SharedSecret` or `SharedConfigMap` resource.

Both layers must be satisfied. If either is missing, the volume mount fails and
the Pod stays in a pending state with a `FailedMount` event.

## Prerequisites

- OpenShift cluster with Shared Resource CSI Driver deployed 
- The CSI driver DaemonSet runs under the `csi-driver-shared-resource`
  ServiceAccount in the `openshift-builds` namespace 

## Granting Driver Permission to Access the Source Resource

The CSI driver no longer has cluster-wide read access to all Secrets and
ConfigMaps. You must explicitly grant it permission to read each source
resource by creating a `Role` and `RoleBinding` in the source namespace.

### For a Secret

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: csi-read-my-secret
  namespace: source-namespace
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: ["my-secret"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: csi-read-my-secret
  namespace: source-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: csi-read-my-secret
subjects:
  - kind: ServiceAccount
    name: csi-driver-shared-resource
    namespace: openshift-builds
```

### For a ConfigMap

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: csi-read-my-configmap
  namespace: source-namespace
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["my-configmap"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: csi-read-my-configmap
  namespace: source-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: csi-read-my-configmap
subjects:
  - kind: ServiceAccount
    name: csi-driver-shared-resource
    namespace: openshift-builds
```

## Granting Pod Permission to Use a Shared Resource

The consuming Pod's ServiceAccount needs the `use` verb on the specific
`SharedSecret` or `SharedConfigMap`. This is done with a `ClusterRole` (since
shared resources are cluster-scoped) and a `RoleBinding` in the consuming
namespace.

### For a SharedSecret

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: use-my-shared-secret
rules:
  - apiGroups:
      - sharedresource.openshift.io
    resources:
      - sharedsecrets
    resourceNames:
      - my-shared-secret
    verbs:
      - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: use-my-shared-secret
  namespace: consuming-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: use-my-shared-secret
subjects:
  - kind: ServiceAccount
    name: default
    namespace: consuming-namespace
```

### For a SharedConfigMap

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: use-my-shared-configmap
rules:
  - apiGroups:
      - sharedresource.openshift.io
    resources:
      - sharedconfigmaps
    resourceNames:
      - my-shared-configmap
    verbs:
      - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: use-my-shared-configmap
  namespace: consuming-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: use-my-shared-configmap
subjects:
  - kind: ServiceAccount
    name: default
    namespace: consuming-namespace
```

## Complete Examples

For full end-to-end walkthroughs that include all RBAC objects, see:

- [Simple example](simple-example.md) — mounts a `SharedConfigMap` into a Pod
  ([YAML files](../examples/simple))
- [Tekton example](tekton-example.md) — mounts a `SharedSecret` into a Tekton
  TaskRun ([YAML files](../examples/tekton))
- [README — How To Use](../README.md#how-to-use) — step-by-step with inline
  YAML covering both driver and pod RBAC

## Troubleshooting

### Mount fails with PermissionDenied

```
FailedMount: rpc error: code = PermissionDenied desc = subjectaccessreviews
share <name> podNamespace <ns> podName <pod> podSA <sa> returned forbidden
```

**Cause:** The Pod's ServiceAccount does not have `use` permission on the
SharedSecret or SharedConfigMap.

**Fix:** Create or verify the `ClusterRole` and `RoleBinding` granting the `use`
verb to the Pod's ServiceAccount as shown above.

### Mount fails with "not found" for the backing resource

```
FailedMount: rpc error: code = Internal desc = failed to populate mount device:
... secrets "my-secret" not found
```

**Cause:** The CSI driver's ServiceAccount does not have permission to read the
source Secret or ConfigMap in its namespace, or the source resource does not
exist.

**Fix:**
1. Verify the source Secret/ConfigMap exists in the expected namespace.
2. Create or verify the `Role` and `RoleBinding` in the source namespace
   granting the CSI driver ServiceAccount `get`, `list`, `watch` on the
   specific resource.

### Data disappears from a running Pod

**Cause:** The RBAC permissions were removed after the Pod started. The CSI
driver periodically re-evaluates permissions (default interval: 10 minutes via
`shareRelistInterval`) and removes data when access is revoked.

**Fix:** Restore the required `Role`/`RoleBinding`. The data will be
re-populated on the next relist cycle. See the [FAQ](faq.md) for details.

## Summary

| RBAC Layer | Scope | Resource | Verbs | Purpose |
|---|---|---|---|---|
| Driver access | Source namespace | `secrets` or `configmaps` | `get`, `list`, `watch` | Allows the CSI driver to read the source data |
| Pod access | Consuming namespace | `sharedsecrets` or `sharedconfigmaps` | `use` | Allows the Pod's SA to mount the shared resource |
