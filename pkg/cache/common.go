package cache

import (
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	storagev1alpha1 "github.com/openshift/api/storage/v1alpha1"
)

// GetNamespaceAndNameFrom returns the namespace and name from a storagev1alpha1.ResourceReference
// regardless of what type it is
func GetNamespaceAndNameFrom(ref storagev1alpha1.ResourceReference) (string, string) {
	var namespace, name string

	switch ref.Type {
	case storagev1alpha1.ResourceReferenceTypeConfigMap:
		namespace, name = ref.ConfigMap.Namespace, ref.ConfigMap.Name
	case storagev1alpha1.ResourceReferenceTypeSecret:
		namespace, name = ref.Secret.Namespace, ref.Secret.Name
	}

	return namespace, name
}

// GetKeyFrom returns a key of the format namespace:name
// from either a metav1.Object or storagev1alpha1.ResourceReference
func GetKeyFrom(o interface{}) string {
	if obj, ok := o.(metav1.Object); ok {
		return BuildKeyUsing(obj.GetNamespace(), obj.GetName())
	} else if obj, ok := o.(storagev1alpha1.ResourceReference); ok {
		return BuildKeyUsing(GetNamespaceAndNameFrom(obj))
	} else {
		return fmt.Sprintf("%s", o)
	}
}

// BuildKeyUsing returns a key of the format namespace:name given a namespace
// and a name
func BuildKeyUsing(namespace, name string) string {
	return fmt.Sprintf("%s:%s", namespace, name)
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
