package e2e

import (
	"context"
	"testing"
	"time"

	sharev1clientset "github.com/openshift/client-go/sharedresource/clientset/versioned"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/test/framework"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

var (
	kubeClient kubernetes.Interface
)

func waitForCRDs(t *testing.T, crdNames ...string) {
	config, _ := client.GetConfig()

	apiExtClient, _ := apiextensionsclientset.NewForConfig(config) // type-safe client for CRDs

	for _, crdName := range crdNames {
		t.Logf("Waiting for CRD %s to be established...", crdName)
		err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			crd, err := apiExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
			if err != nil {
				return false, nil // retrying
			}

			for _, cond := range crd.Status.Conditions {
				if cond.Type == "Established" && cond.Status == "True" {
					return true, nil
				}
			}
			return false, nil // retrying
		})
		if err != nil {
			t.Fatalf("CRD %s was not established within the timeout: %v", crdName, err)
		}
	}
}

func waitForCSIDriverRegistration(t *testing.T, driverName string) {
	t.Logf("Waiting for CSIDriver object %s to be registered...", driverName)
	csiDriverClient := kubeClient.StorageV1().CSIDrivers()

	err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := csiDriverClient.Get(ctx, driverName, metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				t.Logf("CSIDriver %s not found yet. Retrying...", driverName)
				return false, nil
			}
			t.Logf("Error getting CSIDriver %s: %v. Retrying...", driverName, err)
			return false, nil
		}
		t.Logf("CSIDriver object %s found and registered.", driverName)
		return true, nil
	})

	if err != nil {
		t.Fatalf("CSIDriver %s was not registered within the timeout: %v", driverName, err)
	}
}

func createTestNamespace(t *testing.T, baseName string) string {
	namespaceName := framework.GenerateName(baseName)
	t.Logf("Creating test namespace %s", namespaceName)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	_, err := kubeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create test namespace %s: %v", namespaceName, err)
	}
	return namespaceName
}

func runSecretRBAC(t *testing.T, grantDriverRBAC, grantPodRBAC, expectPodUp bool) {
	args := &framework.TestArgs{T: t}
	framework.SetupClientsOutsideTestNamespace(args)
	kubeClient = framework.KubeClient()

	waitForCSIDriverRegistration(t, "csi.sharedresource.openshift.io")

	waitForCRDs(t, "sharedsecrets.sharedresource.openshift.io", "sharedconfigmaps.sharedresource.openshift.io")

	config, _ := client.GetConfig()
	shareClient, err := sharev1clientset.NewForConfig(config)
	if err != nil {
		t.Fatalf("Error creating Shared Resource client: %v", err)
	}

	sourceNamespace := createTestNamespace(t, "e2e-source-ns-")
	consumingNamespace := createTestNamespace(t, "e2e-consuming-ns-")
	defer kubeClient.CoreV1().Namespaces().Delete(context.TODO(), sourceNamespace, metav1.DeleteOptions{})
	defer kubeClient.CoreV1().Namespaces().Delete(context.TODO(), consumingNamespace, metav1.DeleteOptions{})

	sourceSecretName := "source-secret-" + framework.GenerateName("")
	sharedSecretName := "shared-secret-" + framework.GenerateName("")

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: sourceSecretName, Namespace: sourceNamespace},
		StringData: map[string]string{"username": "admin"},
	}
	_, err = kubeClient.CoreV1().Secrets(sourceNamespace).Create(context.TODO(), sourceSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source secret: %v", err)
	}

	args.ShareNameOverride = sharedSecretName
	err = framework.CreateTestSharedSecret(args, sharedSecretName, sourceSecretName, sourceNamespace)
	if err != nil {
		t.Fatalf("failed to create shared secret: %v", err)
	}
	defer shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), sharedSecretName, metav1.DeleteOptions{})

	// Poll to ensure the SharedSecret is available before creating a pod that uses it.
	t.Logf("Waiting for SharedSecret %s to become available...", sharedSecretName)
	err = wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := shareClient.SharedresourceV1alpha1().SharedSecrets().Get(ctx, sharedSecretName, metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				t.Logf("SharedSecret %s not found yet. Retrying...", sharedSecretName)
				return false, nil
			}
			return false, err
		}
		t.Logf("SharedSecret %s found.", sharedSecretName)
		return true, nil
	})
	if err != nil {
		t.Fatalf("Timed out waiting for SharedSecret %s to be created: %v", sharedSecretName, err)
	}

	// Create the ClusterRole that defines the 'use' permission for the pod.
	useClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "use-" + sharedSecretName},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{"sharedresource.openshift.io"},
			Resources:     []string{"sharedsecrets"},
			ResourceNames: []string{sharedSecretName},
			Verbs:         []string{"use"},
		}},
	}
	_, err = kubeClient.RbacV1().ClusterRoles().Create(context.TODO(), useClusterRole, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create 'use' cluster role: %v", err)
	}
	defer kubeClient.RbacV1().ClusterRoles().Delete(context.TODO(), useClusterRole.Name, metav1.DeleteOptions{})

	if grantDriverRBAC {
		t.Log("Granting CSI Driver access to the source secret")
		driverRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "driver-access-" + sourceSecretName, Namespace: sourceNamespace},
			Rules: []rbacv1.PolicyRule{{
				Verbs:         []string{"get", "list", "watch"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{sourceSecretName},
			}},
		}
		_, err := kubeClient.RbacV1().Roles(sourceNamespace).Create(context.TODO(), driverRole, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create driver role: %v", err)
		}

		driverRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "driver-binding-" + sourceSecretName, Namespace: sourceNamespace},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "csi-driver-shared-resource", Namespace: "openshift-builds"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: driverRole.Name},
		}
		_, err = kubeClient.RbacV1().RoleBindings(sourceNamespace).Create(context.TODO(), driverRoleBinding, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create driver role binding: %v", err)
		}
	}

	if grantPodRBAC {
		t.Log("Granting Pod Service Account permission to use the shared secret")
		podRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-can-use-share", Namespace: consumingNamespace},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: useClusterRole.Name},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default", Namespace: consumingNamespace}},
		}
		_, err = kubeClient.RbacV1().RoleBindings(consumingNamespace).Create(context.TODO(), podRoleBinding, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create pod role binding: %v", err)
		}
	}

	args.Name = consumingNamespace
	args.TestPodUp = expectPodUp
	framework.CreatePodWithSharedSecret(args)

	if expectPodUp {
		args.SearchString = "username"
		framework.ExecPod(args)
	}
}

func runConfigMapRBAC(t *testing.T, grantDriverRBAC, grantPodRBAC, expectPodUp bool) {
	args := &framework.TestArgs{T: t}
	framework.SetupClientsOutsideTestNamespace(args)
	kubeClient = framework.KubeClient()

	waitForCSIDriverRegistration(t, "csi.sharedresource.openshift.io")

	waitForCRDs(t, "sharedsecrets.sharedresource.openshift.io", "sharedconfigmaps.sharedresource.openshift.io")

	config, _ := client.GetConfig()
	shareClient, err := sharev1clientset.NewForConfig(config)
	if err != nil {
		t.Fatalf("Error creating Shared Resource client: %v", err)
	}

	sourceNamespace := createTestNamespace(t, "e2e-source-ns-")
	consumingNamespace := createTestNamespace(t, "e2e-consuming-ns-")
	defer kubeClient.CoreV1().Namespaces().Delete(context.TODO(), sourceNamespace, metav1.DeleteOptions{})
	defer kubeClient.CoreV1().Namespaces().Delete(context.TODO(), consumingNamespace, metav1.DeleteOptions{})

	sourceCMName := "source-cm-" + framework.GenerateName("")
	sharedCMName := "shared-cm-" + framework.GenerateName("")

	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: sourceCMName, Namespace: sourceNamespace},
		Data:       map[string]string{"test-key": "test-value"},
	}
	_, err = kubeClient.CoreV1().ConfigMaps(sourceNamespace).Create(context.TODO(), sourceCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source configmap: %v", err)
	}

	args.ShareNameOverride = sharedCMName
	err = framework.CreateTestSharedConfigMap(args, sharedCMName, sourceCMName, sourceNamespace)
	if err != nil {
		t.Fatalf("failed to create shared configmap: %v", err)
	}
	defer shareClient.SharedresourceV1alpha1().SharedConfigMaps().Delete(context.TODO(), sharedCMName, metav1.DeleteOptions{})

	// Poll to ensure the SharedConfigMap is available before creating a pod that uses it.
	t.Logf("Waiting for SharedConfigMap %s to become available...", sharedCMName)
	err = wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Get(ctx, sharedCMName, metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				t.Logf("SharedConfigMap %s not found yet. Retrying...", sharedCMName)
				return false, nil
			}
			return false, err
		}
		t.Logf("SharedConfigMap %s found.", sharedCMName)
		return true, nil
	})
	if err != nil {
		t.Fatalf("Timed out waiting for SharedConfigMap %s to be created: %v", sharedCMName, err)
	}

	// Create the ClusterRole that defines the 'use' permission for the pod.
	useClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "use-" + sharedCMName},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{"sharedresource.openshift.io"},
			Resources:     []string{"sharedconfigmaps"},
			ResourceNames: []string{sharedCMName},
			Verbs:         []string{"use"},
		}},
	}
	_, err = kubeClient.RbacV1().ClusterRoles().Create(context.TODO(), useClusterRole, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create 'use' cluster role: %v", err)
	}
	defer kubeClient.RbacV1().ClusterRoles().Delete(context.TODO(), useClusterRole.Name, metav1.DeleteOptions{})

	if grantDriverRBAC {
		t.Log("Granting CSI Driver access to the source configmap")
		driverRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "driver-access-" + sourceCMName, Namespace: sourceNamespace},
			Rules: []rbacv1.PolicyRule{{
				Verbs:         []string{"get", "list", "watch"},
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{sourceCMName},
			}},
		}
		_, err := kubeClient.RbacV1().Roles(sourceNamespace).Create(context.TODO(), driverRole, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create driver role: %v", err)
		}

		driverRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "driver-binding-" + sourceCMName, Namespace: sourceNamespace},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "csi-driver-shared-resource", Namespace: "openshift-builds"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: driverRole.Name},
		}
		_, err = kubeClient.RbacV1().RoleBindings(sourceNamespace).Create(context.TODO(), driverRoleBinding, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create driver role binding: %v", err)
		}
	}

	if grantPodRBAC {
		t.Log("Granting Pod Service Account permission to use the shared configmap")
		podRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-can-use-share", Namespace: consumingNamespace},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: useClusterRole.Name},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default", Namespace: consumingNamespace}},
		}
		_, err = kubeClient.RbacV1().RoleBindings(consumingNamespace).Create(context.TODO(), podRoleBinding, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create pod role binding: %v", err)
		}
	}

	args.Name = consumingNamespace
	args.TestPodUp = expectPodUp
	framework.CreatePodWithSharedConfigMap(args)

	if expectPodUp {
		args.SearchString = "test-key"
		framework.ExecPod(args)
	}
}

func TestSecret_Success(t *testing.T) {
	runSecretRBAC(t, true, true, true) // mount should succeeds when all permissions are correct.
}

func TestSecret_Failure_MissingDriverRBAC(t *testing.T) {
	runSecretRBAC(t, false, true, false) // mount should fail if the csi-driver lacks permission to the source secret.
}

func TestSecret_Failure_MissingPodRBAC(t *testing.T) {
	runSecretRBAC(t, true, false, false) // mount should fail if the pod's service account lacks 'use' permission.
}

func TestConfigMap_Success(t *testing.T) {
	runConfigMapRBAC(t, true, true, true) // mount should succeed when all permissions are correct.
}

func TestConfigMap_Failure_MissingDriverRBAC(t *testing.T) {
	runConfigMapRBAC(t, false, true, false) // mount should fail if the csi-driver lacks permission to the source configmap.
}

func TestConfigMap_Failure_MissingPodRBAC(t *testing.T) {
	runConfigMapRBAC(t, true, false, false) // mount should fail if the pod's service account lacks 'use' permission.
}
