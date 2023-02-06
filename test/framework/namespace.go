package framework

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	maxNameLength          = 63
	randomLength           = 5
	maxGeneratedNameLength = maxNameLength - randomLength
)

func generateName(base string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))

}

func CreateTestNamespace(t *TestArgs) string {
	testNamespace := generateName("e2e-test-csi-driver-shared-resource-")
	t.T.Logf("%s: Creating test namespace %s", time.Now().String(), testNamespace)
	ns := &corev1.Namespace{}
	ns.Name = testNamespace
	ns.Labels = map[string]string{"openshift.io/cluster-monitoring": "true"}
	_, err := namespaceClient.Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test namespace: %s", err.Error())
	}
	t.T.Logf("%s: Test namespace %s created", time.Now().String(), testNamespace)
	t.Name = testNamespace
	t.SecondName = testNamespace + secondShareSuffix
	return testNamespace
}

func CleanupTestNamespaceAndClusterScopedResources(t *TestArgs) {
	t.T.Logf("%s: start cleanup of test namespace %s", time.Now().String(), t.Name)
	err := shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", t.Name, err.Error())
	}
	t.T.Logf("%s: start cleanup of test namespace %s", time.Now().String(), t.Name)
	err = shareClient.SharedresourceV1alpha1().SharedConfigMaps().Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", t.Name, err.Error())
	}
	err = shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), t.SecondName, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", t.SecondName, err.Error())
	}
	err = shareClient.SharedresourceV1alpha1().SharedConfigMaps().Delete(context.TODO(), t.SecondName, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", t.SecondName, err.Error())
	}
	if len(t.ShareNameOverride) > 0 {
		err = shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), t.ShareNameOverride, metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			t.T.Fatalf("error deleting share %s: %s", t.ShareNameOverride, err.Error())
		}
		err = shareClient.SharedresourceV1alpha1().SharedConfigMaps().Delete(context.TODO(), t.ShareNameOverride, metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			t.T.Fatalf("error deleting share %s: %s", t.ShareNameOverride, err.Error())
		}
	}
	err = shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), "openshift-etc-pki-entitlement", metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share openshift-etc-pki-entitlement: %s", err.Error())
	}
	// try to clean up pods in test namespace individually to avoid weird k8s timing issues
	podList, _ := kubeClient.CoreV1().Pods(t.Name).List(context.TODO(), metav1.ListOptions{})
	for _, pod := range podList.Items {
		err = kubeClient.CoreV1().Pods(t.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			// don't error out
			t.T.Logf("error deleting test pod %s: %s", pod.Name, err.Error())
		}
	}
	err = namespaceClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting test namespace %s: %s", t.Name, err.Error())
	}
	t.T.Logf("%s: cleanup of test namespace %s completed", time.Now().String(), t.Name)
}
