Feature: In this feature we will try to create a Shared Secrets with the name "openshift-etc-pki-entitlement"
    with secretRef other than "etc-pki-entitlement" from the "openshift-config-managed" namespace which should not be allowed.
    No users should be able to create shared Secrets which violates the reserved names list

    @automated @negative
    Scenario: User should not be able to create a Shared Secrets with the name "openshift-etc-pki-entitlement"
    with secretRef other than "etc-pki-entitlement" from the "openshift-config-managed" namespace : CSI-04-TC01
        Given user has cluster scoped level permission to create CRD "sharedsecrets.sharedresource.openshift.io"
        When user creates a SharedSecrets with name "openshift-etc-pki-entitlement" with below secretRef from the "openshift-config-managed" namespace using the file smoke/features/data/invalid-reserved-sharedsecret-name.yaml
        |secretRef                              |
        |kube-controller-manager-client-cert-key|
        |kube-scheduler-client-cert-key         |
        |router-certs                           |
        Then Not allowed to create SharedSecrets with name "openshift-etc-pki-entitlement" as it violates the reserved names list, Error message is received