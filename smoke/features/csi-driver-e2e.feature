Feature: SharedSecrets and SharedConfigMap

    As a user of openshift tech preview feature
    I want to verify the basic functionality of the shared resource csi driver
    To assure the stability in long run

    Background:
        Given We have an openshift techpreview cluster
        And Shared resource csi driver is installed
        Then create a project

    @automated
    Scenario: Configmaps with and without refreshResources are accessbile throughout the cluster : CSI-01-TC01
        Given user has cluster scoped level permission to create CRD "sharedconfigmaps.sharedresource.openshift.io"
        When user creates the configmap "share-config" in a given namespace  
        And defines shared configmap "my-shared-config" that references the "shared-config" configmap from the first project to share across all namespace
        And creates another project that will access the cluster scoped shared configmap that references the "shared-config" in the first project
        And RBAC for the service account to use the "sharedconfigmap" in its pod
        And creates a pod "my-csi-app-check" with a CSI volume citing the shared resource csi driver and requesting the previously defined "sharedconfigmap" in the Pod CSI volume's volume attributes
        And edits configMap "share-config" data test4 from the first project
        Then pod "my-csi-app-check" in the second project should mount the data test4 available in the "share-config"
        When user adds "refreshResources" to "false" in "share-config" configmap
        And edits configMap "share-config" data test5 from the first project
        Then pod "my-csi-app-check" in the second project should not mount the data test5 available in the "share-config"

    @manual
    Scenario: Verify csi metrics using console : CSI-01-TC02
        Given user has created <"resource type"> with <"shared resource">
            | resource type | shared resource    |
            | configmap     | "sharedconfigmaps" |
            |  secret       | "sharedsecrets"    |
        And creates rbac for the service account to use the shared resource in the pod
        And creates a pod "my-csi-app-check" that used the sharedconfigmaps or sharedsecrets
        When user login to openshift console with "kubeadmin" user credential
        And go to "Administrator"
        And clicks on "Observe"
        And clicks on "metrics"
        And search for csi metrics
            | csi metrics                                |
            | "openshift_csi_share_configmap"            |
            | "openshift_csi_share_secret"               |
            | "openshift_csi_share_mount_failures_total" |
            | "openshift_csi_share_mount_requests_total" |
        Then the metrics value for configured resources should be "1"
    
    @manual
    Scenario: shared resource data gets removed if permission removed : CSI-01-TC03
        Given user has created <"resource type"> with <"shared resource">
            | resource type | shared resource    |
            | configmap     | "sharedconfigmaps" |
            |  secret       | "sharedsecrets"    |
        And creates another project that will access the cluster scoped shared resource type that references the resource in the first project
            | resource type | resource       |
            | configmap     | "share-config" |
            |  secret       | "my-secret"    |
        And creates rbac for the service account to use the shared resource in the pod
        And creates a pod "my-csi-app-check" that used the sharedconfigmaps or sharedsecrets
        Then pod "my-csi-app-check" should mount the data available in the shared resource
        When user removes the share related rbac "use" permissions
        Then pod "my-csi-app-check" in the second project should remove the share related data from its volume

    @automated
    Scenario: Secrets are accessbile throughout the cluster : CSI-01-TC04
        Given user has cluster scoped level permission to create CRD "sharedsecrets.sharedresource.openshift.io"
        When user creates the secret "my-secret" in a given namespace 
        And defines shared secret "my-shared-secret" that references the "my-secret" secret from the first project to share across all namespace
        And creates another project that will access the cluster scoped shared secret that references the "my-secret" in the first project
        And RBAC for the service account to use the "sharedsecret" in its pod
        And creates a pod "my-csi-app-check" with a CSI volume citing the shared resource csi driver and requesting the previously defined "sharedsecret" in the Pod CSI volume's volume attributes
        And edits secret "my-secret" data from the first project
        Then pod "my-csi-app-check" in the second project should mount the data "hostpath" available in the "my-secret"