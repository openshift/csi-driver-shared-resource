package cache

import (
	"fmt"
	"k8s.io/klog/v2"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sharev1alpha1 "github.com/openshift/csi-driver-shared-resource/pkg/api/sharedresource/v1alpha1"
)

func GetKey(o interface{}) string {
	obj, ok := o.(metav1.Object)
	if !ok {
		return fmt.Sprintf("%s", o)
	}

	return obj.GetNamespace() + ":" + obj.GetName()
}

func BuildKey(r sharev1alpha1.ResourceReference) string {
	switch r.Type {
	case sharev1alpha1.ResourceReferenceTypeConfigMap:
		return r.ConfigMap.Namespace + ":" + r.ConfigMap.Name
	case sharev1alpha1.ResourceReferenceTypeSecret:
		return r.Secret.Namespace + ":" + r.Secret.Name
	default:
		klog.Warningf("BuildKey unknown type %s", r.Type)
		return ""
	}
}

func GetResourceNamespace(r sharev1alpha1.ResourceReference) string {
	switch r.Type {
	case sharev1alpha1.ResourceReferenceTypeConfigMap:
		return r.ConfigMap.Namespace
	case sharev1alpha1.ResourceReferenceTypeSecret:
		return r.Secret.Namespace
	default:
		klog.Warningf("GetResourceNamespace unknown type %s", r.Type)
		return ""
	}
}

func GetResourceName(r sharev1alpha1.ResourceReference) string {
	switch r.Type {
	case sharev1alpha1.ResourceReferenceTypeConfigMap:
		return r.ConfigMap.Name
	case sharev1alpha1.ResourceReferenceTypeSecret:
		return r.Secret.Name
	default:
		klog.Warningf("GetResourceName unknown type %s", r.Type)
		return ""
	}
}

// SplitKey splits the shared-data-key into namespace and name.
func SplitKey(key string) (string, string, error) {
	s := strings.Split(key, ":")
	if len(s) != 2 || s[0] == "" || s[1] == "" {
		return "", "", fmt.Errorf("unable to split key '%s' into namespace and name", key)
	}
	return s[0], s[1], nil
}

func buildCallbackMap(key, value interface{}) *sync.Map {
	c := &sync.Map{}
	c.Store(key, value)
	return c
}

func buildRanger(m *sync.Map) func(key, value interface{}) bool {
	return func(key, value interface{}) bool {
		f, _ := value.(func(key, value interface{}) bool)
		m.Range(f)
		return true
	}
}
