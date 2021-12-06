Feature: Verify the shared resource CSI driver works on Openshift

    As an application developer using OpenShift
    I want to verify the projected resource CSI driver shares work on OpenShift
    So that I am confident that I can use the shared resources in any workload.

    Scenario: Verify the shared resource CSI driver works on OpenShift
        Given we have a openshift tech-preview cluster