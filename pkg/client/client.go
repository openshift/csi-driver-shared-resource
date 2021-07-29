package client

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ktypedclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
)

const (
	// TODO eventually change to the CSO csi driver namespace when shared-resources-operator is fully integrated into CSO and OCP
	DefaultNamespace = "csi-driver-shared-resource"
)

var (
	initLock   = sync.Mutex{}
	kubeClient kubernetes.Interface
	recorder   record.EventRecorder
)

// SetClient sets the internal kubernetes client interface. Useful for testing.
func SetClient(client kubernetes.Interface) {
	kubeClient = client
}

func GetRecorder() record.EventRecorder {
	return recorder
}

// GetConfig creates a *rest.Config for talking to a Kubernetes apiserver.
// Otherwise will assume running in cluster and use the cluster provided kubeconfig.
//
// Config precedence
//
// * KUBECONFIG environment variable pointing at a file
//
// * In-cluster config if running in cluster
//
// * $HOME/.kube/config if exists
func GetConfig() (*rest.Config, error) {
	// If an env variable is specified with the config locaiton, use that
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	}
	// If no explicit location, try the in-cluster config
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not locate a kubeconfig")
}

func initClient() error {
	initLock.Lock()
	defer initLock.Unlock()
	if kubeClient == nil {
		kubeRestConfig, err := GetConfig()
		if err != nil {
			return err
		}
		kubeClient, err = kubernetes.NewForConfig(kubeRestConfig)
		if err != nil {
			return err
		}

	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&ktypedclient.EventSinkImpl{Interface: kubeClient.CoreV1().Events(DefaultNamespace)})
	recorder = eventBroadcaster.NewRecorder(runtime.NewScheme(), corev1.EventSource{Component: DefaultNamespace})
	return nil
}

func ExecuteSAR(shareName, podNamespace, podName, podSA string) (bool, error) {
	err := initClient()
	if err != nil {
		return false, err
	}
	sarClient := kubeClient.AuthorizationV1().SubjectAccessReviews()
	resourceAttributes := &authorizationv1.ResourceAttributes{
		Verb:     "get",
		Group:    "sharedresource.openshift.io",
		Resource: "shares",
		Name:     shareName,
	}
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes,
			User:               fmt.Sprintf("system:serviceaccount:%s:%s", podNamespace, podSA),
		}}

	resp, err := sarClient.Create(context.TODO(), sar, metav1.CreateOptions{})
	if err == nil && resp != nil {
		if resp.Status.Allowed {
			return true, nil
		}
		return false, status.Errorf(codes.PermissionDenied,
			"subjectaccessreviews share %s podNamespace %s podName %s podSA %s returned forbidden",
			shareName, podNamespace, podName, podSA)
	}

	if kerrors.IsForbidden(err) {
		return false, status.Errorf(codes.PermissionDenied,
			"subjectaccessreviews share %s podNamespace %s podName %s podSA %s returned forbidden: %s",
			shareName, podNamespace, podName, podSA, err.Error())
	}

	return false, status.Errorf(codes.Internal,
		"subjectaccessreviews share %s podNamespace %s podName %s podSA %s returned error: %s",
		shareName, podNamespace, podName, podSA, err.Error())
}

func GetPod(namespace, name string) (*corev1.Pod, error) {
	initClient()
	return kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
