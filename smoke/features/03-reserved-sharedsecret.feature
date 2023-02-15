Feature: Reserved Name Prefix for SharedSecret
    As a user of an OpenShift Cluster I should be able to create a Shared Secrets with the name "openshift-etc-pki-entitlement"
    with secretRef "etc-pki-entitlement" from the "openshift-config-managed" namespace
    This sharedsecret name as well as their reference's namespace and secretRef, are part of the pre-approved list curated in the shared resource operator

    @automated
    Scenario: User should be able to create a Shared Secrets with the name "openshift-etc-pki-entitlement"
    with secretRef "etc-pki-entitlement" from the "openshift-config-managed" namespace : CSI-03-TC01
        Given user has cluster scoped level permission to create CRD "sharedsecrets.sharedresource.openshift.io"
        When user creates a SharedSecrets with name "openshift-etc-pki-entitlement" with secretRef "etc-pki-entitlement" from the "openshift-config-managed" namespace using the file smoke/features/data/valid-reserved-sharedsecret-name.yaml
        Then "sharedsecret.sharedresource.openshift.io/openshift-etc-pki-entitlement" created should be the output