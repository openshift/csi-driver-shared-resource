package cache

import (
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetKey(o interface{}) string {
	obj, ok := o.(metav1.Object)
	if !ok {
		return fmt.Sprintf("%s", o)
	}

	return obj.GetNamespace() + ":" + obj.GetName()
}

func BuildKey(namespace, name string) string {
	return namespace + ":" + name
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
