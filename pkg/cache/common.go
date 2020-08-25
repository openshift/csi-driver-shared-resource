package cache

import (
	"fmt"
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
