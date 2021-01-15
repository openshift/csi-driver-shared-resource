package framework

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"

	kubeset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"

	"github.com/openshift/csi-driver-projected-resource/pkg/client"
	shareset "github.com/openshift/csi-driver-projected-resource/pkg/generated/clientset/versioned"
)

var (
	kubeConfig               *rest.Config
	kubeClient               *kubeset.Clientset
	podClient                corev1client.PodInterface
	restClient               *rest.RESTClient
	namespaceClient          corev1client.NamespaceInterface
	clusterRoleClient        rbacv1client.ClusterRoleInterface
	clusterRoleBindingClient rbacv1client.ClusterRoleBindingInterface
	shareClient              shareset.Interface
)

func SetupClients(t *testing.T) {
	var err error
	if kubeConfig == nil {
		kubeConfig, err = client.GetConfig()
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}
	if kubeClient == nil {
		kubeClient, err = kubeset.NewForConfig(kubeConfig)
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}
	if restClient == nil {
		restClient, err = rest.RESTClientFor(setRESTConfigDefaults(*kubeConfig))
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}
	if namespaceClient == nil {
		namespaceClient = kubeClient.CoreV1().Namespaces()
	}
	if podClient == nil {
		podClient = kubeClient.CoreV1().Pods(client.DefaultNamespace)
	}
	if clusterRoleClient == nil {
		clusterRoleClient = kubeClient.RbacV1().ClusterRoles()
	}
	if clusterRoleBindingClient == nil {
		clusterRoleBindingClient = kubeClient.RbacV1().ClusterRoleBindings()
	}
	shareClient, err = shareset.NewForConfig(kubeConfig)
	if err != nil {
		t.Fatalf("%#v", err)
	}
}

func setRESTConfigDefaults(config rest.Config) *rest.Config {
	if config.GroupVersion == nil {
		config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = scheme.Codecs
	}
	if len(config.UserAgent) == 0 {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	config.APIPath = "/api"
	return &config
}
