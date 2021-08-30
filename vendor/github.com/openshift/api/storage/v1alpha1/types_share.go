package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedResource allows a backing Secret or ConfigMap to be shared across namespaces.
// Pods can mount the shared Secret or ConfigMap by adding a CSI volume to the pod specification using the
// "sharedresource.csi.storage.openshift.io" CSI driver and a reference to the SharedResource in the volume attributes:
//
// spec:
//  volumes:
//  - name: shared-secret
//    csi:
//      driver: sharedresource.csi.storage.openshift.io
//      volumeAttributes:
//        sharedResource: my-share
//
// For the mount to be successful, the pod's service account must be granted permission to get the named SharedResource object with an appropriate
// ClusterRole and ClusterRoleBinding.
//
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +k8s:openapi-gen=true
// +openshift:compatibility-gen:level=4
// +kubebuilder:subresource:status
//
type SharedResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired shared resource
	// +kubebuilder:validation:Required
	Spec SharedResourceSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Observed status of the shared resource
	Status SharedResourceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedResourceList contains a list of SharedResource objects.
//
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type SharedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []SharedResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SharedResourceSpec defines the desired state of a SharedResource.
// +k8s:openapi-gen=true
type SharedResourceSpec struct {
	// resource references the backing object for this shared resource.
	// +kubebuilder:validation:Required
	Resource ResourceReference `json:"resource" protobuf:"bytes,1,name=resource"`

	// description is a user readable explanation of what the backing resource provides.
	Description string `json:"description,omitempty" protobuf:"bytes,2,opt,name=description"`
}

// SharedResourceStatus contains the observed status of shared resource. Read-only.
type SharedResourceStatus struct {
	// conditions represents any observations made on this particular shared resource by the underlying CSI driver or Share controller.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" protobuf:"bytes,1,rep,name=conditions"`
}

// ResourceReferenceType represents a shared resource type
type ResourceReferenceType string

const (
	// ResourceReferenceTypeConfigMap is the ConfigMap shared resource type
	ResourceReferenceTypeConfigMap ResourceReferenceType = "ConfigMap"
	// ResourceReferenceTypeSecret is the Secret shared resource type
	ResourceReferenceTypeSecret ResourceReferenceType = "Secret"
)

// ResourceReference represents the backing object for this share
// Only one of its supported types may be specified at any given time
type ResourceReference struct {
	// type is the SharedResourceType for the shared resource.
	// Valid types are: ConfigMap, Secret.
	// +kubebuilder:validation:Enum="ConfigMap";"Secret"
	Type ResourceReferenceType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=ResourceReferenceType"`
	// configMap provides details about the backing object if it is a ConfigMap.
	// If set, type must be "ConfigMap".
	ConfigMap *ResourceReferenceConfigMap `json:"configMap,omitempty" protobuf:"bytes,3,opt,name=configMap"`
	// secret provides details about the backing object if it is a Secret.
	// If set, type must be "Secret".
	Secret *ResourceReferenceSecret `json:"secret,omitempty" protobuf:"bytes,2,opt,name=secret"`
}

// ResourceReferenceConfigMap provides details about the ConfigMap that is being shared
type ResourceReferenceConfigMap struct {
	// name represents the name of the ConfigMap that is being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name" protobuf:"bytes,1,name=name"`
	// namespace represents the namespace where the referenced ConfigMap is located.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace" protobuf:"bytes,2,name=namespace"`
}

// ResourceReferenceSecret provides details about the Secret that is being shared
type ResourceReferenceSecret struct {
	// name represents the name of the Secret that is being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name" protobuf:"bytes,1,name=name"`
	// namespace represents the namespace where the referenced Secret is located.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace" protobuf:"bytes,2,name=namespace"`
}
