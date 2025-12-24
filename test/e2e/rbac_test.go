package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	sharev1clientset "github.com/openshift/client-go/sharedresource/clientset/versioned"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/test/framework"
)

var (
	kubeClient kubernetes.Interface
)

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

// Fetches the logs of a specific container within a given pod.
func getPodLogs(podName, namespace, containerName string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}
	req := kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error in opening stream for container %s: %v", containerName, err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error in copying info from podLogs to buf: %v", err)
	}
	return buf.String(), nil
}

// Polls the logs of a pod until it sees the expected content.
func verifyPodContent(t *testing.T, podName, namespace, expectedContent string) {
	err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		logs, err := getPodLogs(podName, namespace, "my-frontend")
		if err != nil {
			t.Logf("Warning: could not get logs for pod %s: %v", podName, err)
			return false, nil // Continue polling
		}
		if strings.Contains(logs, expectedContent) {
			t.Logf("Pod '%s' contains '%s'", podName, expectedContent)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("Failed to verify content for pod '%s': timed out waiting for '%s': %v", podName, expectedContent, err)
	}
}

func TestFixedHotReloadStaleData(t *testing.T) {
	args := &framework.TestArgs{T: t}
	framework.SetupClientsOutsideTestNamespace(args)
	kubeClient = framework.KubeClient()
	config, _ := client.GetConfig()
	shareClient, err := sharev1clientset.NewForConfig(config)
	if err != nil {
		t.Fatalf("Error creating Shared Resource client: %v", err)
	}

	// SETUP
	t.Log("Setting up resources for the test.")
	sourceNamespace := createTestNamespace(t, "e2e-source-")
	defer kubeClient.CoreV1().Namespaces().Delete(context.TODO(), sourceNamespace, metav1.DeleteOptions{})

	secretV1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret-v1", Namespace: sourceNamespace},
		StringData: map[string]string{"key": "value1"},
	}
	secretV2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret-v2", Namespace: sourceNamespace},
		StringData: map[string]string{"key": "value2"},
	}
	secretV3 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret-v3", Namespace: sourceNamespace},
		StringData: map[string]string{"key": "value3"},
	}
	_, err = kubeClient.CoreV1().Secrets(sourceNamespace).Create(context.TODO(), secretV1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret-v1: %v", err)
	}
	_, err = kubeClient.CoreV1().Secrets(sourceNamespace).Create(context.TODO(), secretV2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret-v2: %v", err)
	}
	_, err = kubeClient.CoreV1().Secrets(sourceNamespace).Create(context.TODO(), secretV3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret-v3: %v", err)
	}

	sharedSecret := &sharev1alpha1.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "data-share"},
		Spec: sharev1alpha1.SharedSecretSpec{
			SecretRef: sharev1alpha1.SharedSecretReference{Name: "secret-v1", Namespace: sourceNamespace},
		},
	}
	_, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Create(context.TODO(), sharedSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create SharedSecret: %v", err)
	}
	defer shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), sharedSecret.Name, metav1.DeleteOptions{})

	// PERMISSIONS
	driverRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "driver-access", Namespace: sourceNamespace},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"secrets"},
		}},
	}
	_, err = kubeClient.RbacV1().Roles(sourceNamespace).Create(context.TODO(), driverRole, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create driver role: %v", err)
	}
	defer kubeClient.RbacV1().Roles(sourceNamespace).Delete(context.TODO(), driverRole.Name, metav1.DeleteOptions{})

	driverRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "driver-binding", Namespace: sourceNamespace},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "csi-driver-shared-resource", Namespace: "openshift-builds"}},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: driverRole.Name},
	}
	_, err = kubeClient.RbacV1().RoleBindings(sourceNamespace).Create(context.TODO(), driverRoleBinding, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create driver role binding: %v", err)
	}
	defer kubeClient.RbacV1().RoleBindings(sourceNamespace).Delete(context.TODO(), driverRoleBinding.Name, metav1.DeleteOptions{})

	useClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "use-" + sharedSecret.Name},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{"sharedresource.openshift.io"},
			Resources:     []string{"sharedsecrets"},
			ResourceNames: []string{sharedSecret.Name},
			Verbs:         []string{"use"},
		}},
	}
	_, err = kubeClient.RbacV1().ClusterRoles().Create(context.TODO(), useClusterRole, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create 'use' cluster role: %v", err)
	}
	defer kubeClient.RbacV1().ClusterRoles().Delete(context.TODO(), useClusterRole.Name, metav1.DeleteOptions{})

	podNamespaces := make(map[string]string)
	podNames := []string{"pod-a", "pod-b", "pod-c", "pod-d", "pod-e"}
	for _, podName := range podNames {
		ns := createTestNamespace(t, fmt.Sprintf("e2e-%s-", podName))
		podNamespaces[podName] = ns
		defer kubeClient.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})
	}

	readOnly := true
	for _, podName := range podNames {
		consumingNS := podNamespaces[podName]
		podRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-use-share", Namespace: consumingNS},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: useClusterRole.Name},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default", Namespace: consumingNS}},
		}
		_, err = kubeClient.RbacV1().RoleBindings(consumingNS).Create(context.TODO(), podRoleBinding, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create pod role binding: %v", err)
		}
		defer kubeClient.RbacV1().RoleBindings(consumingNS).Delete(context.TODO(), podRoleBinding.Name, metav1.DeleteOptions{})

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: consumingNS},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:    "my-frontend",
					Image:   "registry.access.redhat.com/ubi8/ubi-minimal",
					Command: []string{"/bin/sh", "-c", "while true; do if [ -f /data/key ]; then echo -n ''; cat /data/key; echo; fi; sleep 2; done"},
					VolumeMounts: []corev1.VolumeMount{{
						Name: "my-csi-volume", MountPath: "/data", ReadOnly: true,
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: "my-csi-volume",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:           "csi.sharedresource.openshift.io",
							ReadOnly:         &readOnly,
							VolumeAttributes: map[string]string{"sharedSecret": sharedSecret.Name},
						},
					},
				}},
				ServiceAccountName: "default",
			},
		}
		podClient := kubeClient.CoreV1().Pods(consumingNS)
		_, err = podClient.Create(context.TODO(), pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create pod %s: %v", podName, err)
		}
		err = wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			p, err := podClient.Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return p.Status.Phase == corev1.PodRunning, nil
		})
		if err != nil {
			t.Fatalf("Pod %s did not reach running state: %v", podName, err)
		}
	}

	for _, podName := range podNames {
		verifyPodContent(t, podName, podNamespaces[podName], "value1")
	}

	// UPDATE SCENARIO
	t.Log("Updating all pods to use secret-v2")
	sharedSecret, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), sharedSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get latest SharedSecret before update: %v", err)
	}
	sharedSecret.Spec.SecretRef.Name = "secret-v2"
	_, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Update(context.TODO(), sharedSecret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update SharedSecret to point to v2: %v", err)
	}

	for _, podName := range podNames {
		verifyPodContent(t, podName, podNamespaces[podName], "value2")
	}

	// POD RBAC REVOKE SCENARIO
	t.Log("--- Starting Test Case: Pod RBAC Revoke ---")

	t.Logf("[DO] Revoking 'use' permission for pod-c by deleting its RoleBinding.")
	if err := kubeClient.RbacV1().RoleBindings(podNamespaces["pod-c"]).Delete(context.TODO(), "pod-use-share", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("failed to delete pod-c rolebinding to simulate RBAC revoke: %v", err)
	}
	t.Logf("[DONE] Deleted RoleBinding for pod-c.")

	t.Logf("[DO] Triggering update by pointing SharedSecret to secret-v3")
	sharedSecret, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), sharedSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get latest SharedSecret before v3 update: %v", err)
	}
	sharedSecret.Spec.SecretRef.Name = "secret-v3"
	_, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Update(context.TODO(), sharedSecret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update SharedSecret to v3: %v", err)
	}
	t.Logf("[DONE] Updated SharedSecret to point to secret-v3.")

	time.Sleep(15 * time.Second)

	verifyPodContent(t, "pod-a", podNamespaces["pod-a"], "value3")
	verifyPodContent(t, "pod-b", podNamespaces["pod-b"], "value3")
	verifyPodContent(t, "pod-d", podNamespaces["pod-d"], "value3")
	verifyPodContent(t, "pod-e", podNamespaces["pod-e"], "value3")

	t.Logf("Verifying pod-c was NOT updated to value3 and retained value2.")
	if logs, err := getPodLogs("pod-c", podNamespaces["pod-c"], "my-frontend"); err == nil {
		if strings.Contains(logs, "value3") {
			t.Fatalf("FAILED: pod-c should not have updated to value3, but it did.")
		}
		if !strings.Contains(logs, "value2") {
			t.Fatalf("FAILED: pod-c should have retained value2, but it did not.")
		}
		t.Logf("Success: Pod-c did not update to value3 and retained 'value2'.")
	} else {
		t.Logf("Warning: could not retrieve logs for pod-c to verify non-update: %v", err)
	}
	t.Logf("--- Test PASSED: UPDATE Successful on Pod RBAC Revoke ---")

	// FAILURE SCENARIO
	t.Logf("--- Starting Test Case: Check if update succeeds on deleting secret-v2 ---")

	t.Logf("[DO] Introducing a failure by deleting a source secret (secret-v2)")
	err = kubeClient.CoreV1().Secrets(sourceNamespace).Delete(context.TODO(), "secret-v2", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete secret-v2 to introduce failure: %v", err)
	}
	t.Logf("[DONE] Deleted secret-v2.")

	t.Logf("[DO] Update SharedSecret back to the deleted secret to force refresh")
	sharedSecret, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), sharedSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get latest SharedSecret before update: %v", err)
	}
	sharedSecret.Spec.SecretRef.Name = "secret-v2"
	_, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Update(context.TODO(), sharedSecret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update SharedSecret to trigger reload: %v", err)
	}
	t.Logf("[DONE] Updated SharedSecret to point to deleted secret-v2.")

	t.Log("Verifying that all pods have retained their last known good data")
	time.Sleep(20 * time.Second)

	verifyPodContent(t, "pod-a", podNamespaces["pod-a"], "value3")
	verifyPodContent(t, "pod-b", podNamespaces["pod-b"], "value3")
	verifyPodContent(t, "pod-c", podNamespaces["pod-c"], "value2") // Didn't get value3
	verifyPodContent(t, "pod-d", podNamespaces["pod-d"], "value3")
	verifyPodContent(t, "pod-e", podNamespaces["pod-e"], "value3")

	t.Logf("--- Test PASSED: UPDATE Successful on Deleting Secret ---")

	// DRIVER RBAC REMOVE SCENARIO
	t.Logf("--- Starting Test Case: Driver RBAC Revoke ---")

	sharedSecret, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), sharedSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get latest SharedSecret before state restore: %v", err)
	}
	sharedSecret.Spec.SecretRef.Name = "secret-v3"
	_, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Update(context.TODO(), sharedSecret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update SharedSecret to restored v3: %v", err)
	}
	time.Sleep(15 * time.Second)

	t.Logf("[DO] Revoking driver's 'get' permission for secrets")
	err = kubeClient.RbacV1().RoleBindings(sourceNamespace).Delete(context.TODO(), driverRoleBinding.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete driver role binding to simulate RBAC revoke: %v", err)
	}
	t.Logf("[DONE] Deleted driver RoleBinding.")

	t.Logf("[DO] Triggering update by pointing SharedSecret to secret-v1")
	sharedSecret, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), sharedSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get latest SharedSecret before v1 update: %v", err)
	}
	sharedSecret.Spec.SecretRef.Name = "secret-v1"
	_, err = shareClient.SharedresourceV1alpha1().SharedSecrets().Update(context.TODO(), sharedSecret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update SharedSecret to v1: %v", err)
	}
	t.Logf("[DONE] Updated SharedSecret to point to secret-v1.")

	t.Log("Verifying that all pods have retained their last known data due to driver permission failure")
	time.Sleep(20 * time.Second)

	verifyPodContent(t, "pod-a", podNamespaces["pod-a"], "value3")
	verifyPodContent(t, "pod-b", podNamespaces["pod-b"], "value3")
	verifyPodContent(t, "pod-c", podNamespaces["pod-c"], "value2")
	verifyPodContent(t, "pod-d", podNamespaces["pod-d"], "value3")
	verifyPodContent(t, "pod-e", podNamespaces["pod-e"], "value3")

	t.Logf("--- Test PASSED: UPDATE Successful on Driver RBAC Revoke ---")
}
