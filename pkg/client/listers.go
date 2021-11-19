package client

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	sharelisterv1alpha1 "github.com/openshift/client-go/sharedresource/listers/sharedresource/v1alpha1"
)

type Listers struct {
	Secrets          corelistersv1.SecretLister
	ConfigMaps       corelistersv1.ConfigMapLister
	SharedConfigMaps sharelisterv1alpha1.SharedConfigMapLister
	SharedSecrets    sharelisterv1alpha1.SharedSecretLister
}

var singleton Listers

func init() {
	singleton = Listers{}
}

func SetSecretsLister(s corelistersv1.SecretLister) {
	singleton.Secrets = s
}

func SetConfigMapsLister(c corelistersv1.ConfigMapLister) {
	singleton.ConfigMaps = c
}

func SetSharedConfigMapsLister(s sharelisterv1alpha1.SharedConfigMapLister) {
	singleton.SharedConfigMaps = s
}

func SetSharedSecretsLister(s sharelisterv1alpha1.SharedSecretLister) {
	singleton.SharedSecrets = s
}

func GetListers() *Listers {
	return &singleton
}

func GetSecret(namespace, name string) *corev1.Secret {
	if singleton.Secrets != nil {
		s, err := singleton.Secrets.Secrets(namespace).Get(name)
		if err == nil {
			return s
		}
	}
	if kubeClient != nil {
		s, err := kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return s
		}
	}
	return nil
}

func GetConfigMap(namespace, name string) *corev1.ConfigMap {
	if singleton.ConfigMaps != nil {
		cm, err := singleton.ConfigMaps.ConfigMaps(namespace).Get(name)
		if err == nil {
			return cm
		}
	}
	if kubeClient != nil {
		cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return cm
		}
	}
	return nil
}

func GetSharedSecret(name string) *sharev1alpha1.SharedSecret {
	if singleton.SharedSecrets != nil {
		s, err := singleton.SharedSecrets.Get(name)
		if err == nil {
			return s
		}
	}
	if shareClient != nil {
		s, err := shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return s
		}
	}
	return nil
}

func GetSharedConfigMap(name string) *sharev1alpha1.SharedConfigMap {
	if singleton.SharedConfigMaps != nil {
		s, err := singleton.SharedConfigMaps.Get(name)
		if err == nil {
			return s
		}
	}
	if shareClient != nil {
		s, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return s
		}
	}
	return nil
}
