package client

import (
	corev1 "k8s.io/client-go/listers/core/v1"

	storagev1alpha1 "github.com/openshift/client-go/storage/listers/storage/v1alpha1"
)

type Listers struct {
	Secrets    corev1.SecretLister
	ConfigMaps corev1.ConfigMapLister
	Shares     storagev1alpha1.SharedResourceLister
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

func SetSharesLister(s storagev1alpha1.SharedResourceLister) {
	singleton.Shares = s
}

func GetListers() *Listers {
	return &singleton
}
