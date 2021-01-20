package framework

import (
	"context"
	"fmt"
	"testing"
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

func CreateTestNamespace(t *testing.T) string {
	testNamespace := generateName("e2e-test-csi-driver-projected-resource-")
	t.Logf("%s: Creating test namespace %s", time.Now().String(), testNamespace)
	ns := &corev1.Namespace{}
	ns.Name = testNamespace
	ns.Labels = map[string]string{"openshift.io/cluster-monitoring": "true"}
	_, err := namespaceClient.Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("error creating test namespace: %s", err.Error())
	}
	t.Logf("%s: Test namespace %s created", time.Now().String(), testNamespace)
	return testNamespace
}

func CleanupTestNamespace(name string, t *testing.T) {
	t.Logf("%s: start cleanup of test namespace %s", time.Now().String(), name)
	err := clusterRoleBindingClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting cluster role %s: %s", name, err.Error())
	}
	err = clusterRoleClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting cluster role binding %s: %s", name, err.Error())
	}
	err = shareClient.ProjectedresourceV1alpha1().Shares().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting share %s: %s", name, err.Error())
	}
	err = namespaceClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting test namespace %s: %s", name, err.Error())
	}
	t.Logf("%s: cleanup of test namespace %s completed", time.Now().String(), name)
}
