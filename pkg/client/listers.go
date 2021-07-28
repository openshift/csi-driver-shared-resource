package client

import (
	sharev1alpha1 "github.com/openshift/csi-driver-projected-resource/pkg/generated/listers/sharedresource/v1alpha1"
	corev1 "k8s.io/client-go/listers/core/v1"
)

type Listers struct {
	Secrets    corev1.SecretLister
	ConfigMaps corev1.ConfigMapLister
	Shares     sharev1alpha1.ShareLister
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

func SetSharesLister(s sharev1alpha1.ShareLister) {
	singleton.Shares = s
}

func GetListers() *Listers {
	return &singleton
}
