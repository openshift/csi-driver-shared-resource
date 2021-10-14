Feature: SharedSecrets and SharedConfigMap

    As a user of openshift tech preview feature
    I want to verify the functionality of shared resource of csi driver across the defined namespaces
    To assure the stability in long run

    Background:
        Given We have an openshift techpreview cluster
        And Shared resource csi driver is installed
        Then create a project

    @automated
    Scenario: Configmaps are accessbile throughout the cluster : CSI-01-TC01
        Given user has cluster scoped level permission to create CRD "sharedconfigmaps.sharedresource.openshift.io"
        When user creates the configmap "share-config"   
        And defines shared configmap "my-shared-config" to share across all namespace
        And creates another project to access shared resource in different namespace
        And RBAC for the service account to use the "configmap" in its pod
        And creates a pod "my-csi-app-check" with "configmap" volume attributes
        And edits configMap "share-config" data
        Then pod "my-csi-app-check" should reflect the changed "test4"

    @manual
    Scenario: Verify csi metrics using console : CSI-01-TC02
        Given user has created <"resource type"> with <"shared resource">
            | resource type | shared resource    |
            | configmap     | "sharedconfigmaps" |
            |  secret       | "sharedsecrets"    |
        And creates rbac for the service account to use the shared resource in its pod
        And creates a pod "my-csi-app-check"
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

    @automated
    Scenario: Secrets are accessbile throughout the cluster : CSI-01-TC03
        Given user has cluster scoped level permission to create CRD "sharedsecrets.sharedresource.openshift.io"
        When user creates the secret "my-secret"   
        And defines shared secret "my-shared-secret" to share across all namespace
        And creates another project to access shared resource in different namespace
        And RBAC for the service account to use the "secret" in its pod
        And creates a pod "my-csi-app-check" with "secret" volume attributes
        And edits secret "my-secret" data
        Then pod "my-csi-app-check" should reflect the changed "hostpath"
