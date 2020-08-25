package client

import (
	corev1 "k8s.io/client-go/listers/core/v1"
)

type Listers struct {
	Secrets    corev1.SecretNamespaceLister
	ConfigMaps corev1.ConfigMapNamespaceLister
}
