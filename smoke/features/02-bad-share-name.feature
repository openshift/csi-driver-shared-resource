Feature: Reserved Name Prefix for SharedSecret/SharedConfigMaps
    As a user of an OpenShift Cluster I should be restricted from creating random Shared Secrets and ConfigMaps with the "openshift-" prefix
    No users be able to create Shared Secrets and Shared ConfigMaps with prefix that is "openshift-" unless the name they use, 
    as well as their reference's namespace and name, are part of the pre-approved list curated in the shared resource operator

    @automated
    Scenario: User should be restricted from creating random Shared ConfigMaps with the "openshift-" prefix : CSI-02-TC01
        Given user has cluster scoped level permission to create CRD "sharedconfigmaps.sharedresource.openshift.io"
        When user creates a SharedConfigMap with name "openshift-foo" using the file smoke/features/data/bad-shareconfig-name.yaml
        Then Not allowed to create SharedConfigMap with name "openshift-foo" as it violates the reserved names list error message is received

    @automated
    Scenario: User should be restricted from creating random Shared Secrets with the "openshift-" prefix : CSI-02-TC02
        Given user has cluster scoped level permission to create CRD "sharedsecrets.sharedresource.openshift.io"
        When user creates a SharedSecrets with name "openshift-bar" using the file smoke/features/data/bad-sharedsecrets-name.yaml
        Then Not allowed to create SharedSecrets with name "openshift-bar" as it violates the reserved names list error message is received