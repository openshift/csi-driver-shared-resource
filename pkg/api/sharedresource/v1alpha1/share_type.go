package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// ResourceReference represents the ConfigMap or Secret for this share
type ResourceReference struct {
	// name represents the name of the ConfigMap or Secret that is being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// namespace represents the namespace where the referenced ConfigMap or Secret is located.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace" protobuf:"bytes,2,opt,name=namespace"`
}
