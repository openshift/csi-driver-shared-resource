# Configuration

The CSI driver instance is configured by a combination of command-line flags and a YAML configuration
file. You can use the following flags:

- `--config`: path to the YAML configuration file, optional
- `--nodeid`: the current Kubernetes node identifier, must be the same than the node it's running on
- `--endpoint`: CSI driver API endpoint for Kubernetes kubelet
- `--drivername`: CSI driver name to be registered in the cluster
- `--maxvolumespernode`: maximum amount of volumes per node

During the rollout of the CSI driver it captures the node-id via `env.ValueFrom`
[directive](./deploy/csi-hostpath-plugin.yaml).

## Configuration File

By default the CSI driver expects the configuration at `/var/run/configmaps/config/config.yaml` as a
result of the driver's DaemonSet/Pod definition mounting the `csi-driver-shared-resource-config`
ConfigMap from the `openshift-cluster-csi-drivers` namespace, where that file can contain the
following attributes:

```yml
---
# interval to relist all "Share" object instances
shareRelistInterval: 10m

# toggles actively watching for resources, when disabled it will only read objects before mount
refreshResources: true

# list of namespace names ignored
ignoredNamespaces: []
```

When the file is not present, the driver assumes default values instead. And, when the configuration
contents change,  it restarts after a couple second, allowing Kubernetes to restart it back again,
with updated configs.

During the rollout, a `ConfigMap` named `csi-driver-shared-resource-config` is created with default
configuration values, in the same namespace than the CSI driver lives in, i.e.
`openshift-cluster-csi-drivers`.

### Maintaining the ConfigMap

So typically you will  create or update the ConfigMap `csi-driver-shared-resource-config` in the
`openshift-cluster-csi-drivers` namespace using the standard `oc create configmap ...` command with
the `--from-file` option to take the `config.yaml` file described above and inject it into the
`ConfigMap` in question.

As an ease of use assistance for local development, a  `config` Makefile target is provided, and as
the example below shows, it will create the `ConfigMap` using the contents of
[`config/config.yaml`](./config/config.yaml).

```sh
make config
```

You can edit the local configuration file and issue `make config` again, the driver will detect the
new settings and restart. To note, when the driver is deployed before the `ConfigMap` exists, you'll
need to restart the driver's POD manually, so the configuration can be mounted and then subsequent
configuration changes can be detected.
