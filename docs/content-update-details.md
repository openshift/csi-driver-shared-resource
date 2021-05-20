# Details around pushing Secret and ConfigMap updates to provisioned Volumes

### Excluded OCP namespaces

The current list of namespaces excluded from the controller's watches:

- kube-system
- openshift-machine-api
- openshift-kube-apiserver
- openshift-kube-apiserver-operator
- openshift-kube-scheduler
- openshift-kube-controller-manager
- openshift-kube-controller-manager-operator
- openshift-kube-scheduler-operator
- openshift-console-operator
- openshift-controller-manager
- openshift-controller-manager-operator
- openshift-cloud-credential-operator
- openshift-authentication-operator
- openshift-service-ca
- openshift-kube-storage-version-migrator-operator
- openshift-config-operator
- openshift-etcd-operator
- openshift-apiserver-operator
- openshift-cluster-csi-drivers
- openshift-cluster-storage-operator
- openshift-cluster-version
- openshift-image-registry
- openshift-machine-config-operator
- openshift-sdn
- openshift-service-ca-operator

The list is not yet configurable, but most likely will become so as the project's lifecycle progresses.

Allowing the disabling processing of updates, or switching the default for the system as not dealing with
updates, but then allowing for opting into updates, is also under consideration.

Lastly, the current abilities to switch which Secret or ConfigMap a `Share` references, or even switch between
a ConfigMaps and Secrets (and vice-versa of course) is under consideration, and may be removed during these 
still early stages of this driver's lifecycle.
