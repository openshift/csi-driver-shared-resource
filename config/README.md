# config

Deployment manifests for the CSI Driver Shared Resource, laid out in the
controller-runtime / kubebuilder `config/` convention. Each subdirectory is a
self-contained [kustomize](https://kustomize.io/) component, and the top-level
`kustomization.yaml` aggregates them.

## Layout

| Path             | Contents                                                              | Source |
| ---------------- | -------------------------------------------------------------------- | ------ |
| `crd/bases/`     | `SharedConfigMap` / `SharedSecret` CustomResourceDefinitions          | [openshift/api `sharedresource`](https://github.com/openshift/api/tree/master/sharedresource) |
| `rbac/`          | ClusterRoles/Roles and bindings for the node plugin and metrics       | [operator `assets/rbac`](https://github.com/openshift/csi-driver-shared-resource-operator/tree/master/assets/rbac) |
| `driver/`        | Node DaemonSet, CSIDriver, ServiceAccount, config, services, monitor  | [operator `assets`](https://github.com/openshift/csi-driver-shared-resource-operator/tree/master/assets) |
| `webhook/`       | Admission webhook Deployment, Service, PDB, ValidatingWebhookConfig   | [operator `assets/webhook`](https://github.com/openshift/csi-driver-shared-resource-operator/tree/master/assets/webhook) |

The webhook is intentionally kept in its own folder so it can be rendered and
deployed independently of the driver workload.

## Rendering

Render the complete deployment:

```sh
kustomize build config
# or, without a standalone binary:
kubectl kustomize config
```

Render an individual component:

```sh
kustomize build config/crd
kustomize build config/webhook
```

Apply directly:

```sh
kubectl apply -k config
```

## Images

The operator normally templates image references (e.g. `${DRIVER_IMAGE}`) at
runtime. Here those are represented as symbolic image names resolved by the
`images:` transformer in `driver/kustomization.yaml` and
`webhook/kustomization.yaml`. Override `newName`/`newTag` there (or with
`kustomize edit set image ...`) to point at your own builds.
