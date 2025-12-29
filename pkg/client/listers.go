package client

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	sharelisterv1alpha1 "github.com/openshift/client-go/sharedresource/listers/sharedresource/v1alpha1"
)

type Listers struct {
	Secrets          sync.Map
	ConfigMaps       sync.Map
	SharedConfigMaps sharelisterv1alpha1.SharedConfigMapLister
	SharedSecrets    sharelisterv1alpha1.SharedSecretLister
}

var singleton Listers

func init() {
	singleton = Listers{Secrets: sync.Map{}, ConfigMaps: sync.Map{}}
}

func SetSecretsLister(namespace string, s corelistersv1.SecretLister) {
	singleton.Secrets.Store(namespace, s)
}

func SetConfigMapsLister(namespace string, c corelistersv1.ConfigMapLister) {
	singleton.ConfigMaps.Store(namespace, c)
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

func GetSecret(namespace, name string) (*corev1.Secret, error) {
	var lister corelistersv1.SecretLister
	obj, ok := singleton.Secrets.Load(namespace)
	if ok {
		lister = obj.(corelistersv1.SecretLister)
	}
	if lister != nil {
		s, err := lister.Secrets(namespace).Get(name)
		if err == nil {
			return s, nil
		}
		klog.V(4).Infof("GetSecret lister for %s/%s got error: %s", namespace, name, err.Error())
	}
	if kubeClient != nil {
		s, err := kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			// "forbidden" error will be caught and returned
			klog.V(4).Infof("GetSecret client for %s/%s got error: %s", namespace, name, err.Error())
			return nil, err
		}
		return s, nil
	}
	return nil, fmt.Errorf("no secret lister or kubeClient available for namespace %s", namespace)
}

func GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	var lister corelistersv1.ConfigMapLister
	obj, ok := singleton.ConfigMaps.Load(namespace)
	if ok {
		lister = obj.(corelistersv1.ConfigMapLister)
	}
	if lister != nil {
		cm, err := lister.ConfigMaps(namespace).Get(name)
		if err == nil {
			return cm, nil
		}
		klog.V(4).Infof("GetConfigMap lister for %s/%s got error: %s", namespace, name, err.Error())
	}
	if kubeClient != nil {
		cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			// "forbidden" error will be caught and returned
			klog.V(4).Infof("GetConfigMap client for %s/%s got error: %s", namespace, name, err.Error())
			return nil, err
		}
		return cm, nil
	}
	return nil, fmt.Errorf("no configmap lister or kubeClient available for namespace %s", namespace)
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

func ListSharedSecrets() map[string]*sharev1alpha1.SharedSecret {
	ret := map[string]*sharev1alpha1.SharedSecret{}
	if singleton.SharedSecrets != nil {
		list, err := singleton.SharedSecrets.List(labels.Everything())
		if err == nil {
			for _, ss := range list {
				ret[ss.Name] = ss
			}
		}
	}
	if shareClient != nil && len(ret) == 0 {
		list, err := shareClient.SharedresourceV1alpha1().SharedSecrets().List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			for _, ss := range list.Items {
				ret[ss.Name] = &ss
			}
		}
	}
	return ret
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

func ListSharedConfigMap() map[string]*sharev1alpha1.SharedConfigMap {
	ret := map[string]*sharev1alpha1.SharedConfigMap{}
	if singleton.SharedSecrets != nil {
		list, err := singleton.SharedConfigMaps.List(labels.Everything())
		if err == nil {
			for _, scm := range list {
				ret[scm.Name] = scm
			}
		}
	}
	if shareClient != nil && len(ret) == 0 {
		list, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			for _, scm := range list.Items {
				ret[scm.Name] = &scm
			}
		}
	}
	return ret
}
