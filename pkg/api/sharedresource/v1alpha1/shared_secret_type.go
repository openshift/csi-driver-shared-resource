package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedSecret allows a Secret to be shared across namespaces.
// Pods can mount the shared Secret by adding a CSI volume to the pod specification using the
// "csi.sharedresource.openshift.io" CSI driver and a reference to the SharedSecret in the volume attributes:
//
// spec:
//  volumes:
//  - name: shared-secret
//    csi:
//      driver: csi.sharedresource.openshift.io
//      volumeAttributes:
//        sharedSecret: my-share
//
// For the mount to be successful, the pod's service account must be granted permission to 'use' the named SharedSecret object
// within its namespace with an appropriate Role and RoleBinding. For compactness, here are example `oc` invocations for creating
// such Role and RoleBinding objects.
//
//  `oc create role shared-resource-my-share --verb=use --resource=sharedsecrets.sharedresource.openshift.io --resource-name=my-share`
//  `oc create rolebinding shared-resource-my-share --role=shared-resource-my-share --serviceaccount=my-namespace:default`
//
// Administrators can create separate Roles and RoleBindings for their users to be able the list and/or view the
// available cluster scoped SharedSecret objects.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +k8s:openapi-gen=true
// +openshift:compatibility-gen:level=4
// +kubebuilder:subresource:status
//
type SharedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired shared secret
	// +kubebuilder:validation:Required
	Spec SharedSecretSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Observed status of the shared configmap
	Status SharedResourceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedSecretList contains a list of SharedSecret objects.
//
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type SharedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []SharedSecret `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SharedSecretSpec defines the desired state of a SharedSecret
// +k8s:openapi-gen=true
type SharedSecretSpec struct {
	// secret references the backing Secret for this SharedSecret.
	// +kubebuilder:validation:Required
	Secret ResourceReference `json:"secret" protobuf:"bytes,1,opt,name=secret"`

	// description is a user readable explanation of what the backing resource provides.
	Description string `json:"description,omitempty" protobuf:"bytes,2,opt,name=description"`
}
