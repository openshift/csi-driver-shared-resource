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
	testNamespace := generateName("e2e-test-csi-driver-projected-resource-")
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

func CleanupTestNamespace(t *TestArgs) {
	t.T.Logf("%s: start cleanup of test namespace %s", time.Now().String(), t.Name)
	err := clusterRoleBindingClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting cluster role %s: %s", t.Name, err.Error())
	}
	err = clusterRoleClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting cluster role binding %s: %s", t.Name, err.Error())
	}
	err = shareClient.ProjectedresourceV1alpha1().Shares().Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", t.Name, err.Error())
	}
	err = shareClient.ProjectedresourceV1alpha1().Shares().Delete(context.TODO(), t.SecondName, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", t.Name, err.Error())
	}
	err = namespaceClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting test namespace %s: %s", t.Name, err.Error())
	}
	t.T.Logf("%s: cleanup of test namespace %s completed", time.Now().String(), t.Name)
}
