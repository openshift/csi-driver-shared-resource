package client

import (
	sharev1alpha1 "github.com/openshift/client-go/sharedresource/listers/sharedresource/v1alpha1"
	corev1 "k8s.io/client-go/listers/core/v1"
)

type Listers struct {
	Secrets          corev1.SecretLister
	ConfigMaps       corev1.ConfigMapLister
	SharedConfigMaps sharev1alpha1.SharedConfigMapLister
	SharedSecrets    sharev1alpha1.SharedSecretLister
}

var singleton Listers

func init() {
	singleton = Listers{}
}

func SetSecretsLister(s corev1.SecretLister) {
	singleton.Secrets = s
}

func SetConfigMapsLister(c corev1.ConfigMapLister) {
	singleton.ConfigMaps = c
}

func SetSharedConfigMapsLister(s sharev1alpha1.SharedConfigMapLister) {
	singleton.SharedConfigMaps = s
}

func SetSharedSecretsLister(s sharev1alpha1.SharedSecretLister) {
	singleton.SharedSecrets = s
}

func GetListers() *Listers {
	return &singleton
}
