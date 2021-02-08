package framework

import (
	"context"
	"testing"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createShareClusterRole(name string, t *testing.T) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: []rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				Verbs:         []string{"get", "list", "watch"},
				APIGroups:     []string{"projectedresource.storage.openshift.io"},
				Resources:     []string{"shares"},
				ResourceNames: []string{name},
			},
		},
	}
	_, err := clusterRoleClient.Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("error creating test cluster role: %s", err.Error())
	}
}

func createShareClusterRoleBinding(name string, t *testing.T) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
	}
	_, err := clusterRoleBindingClient.Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("error creating test cluster role binding: %s", err.Error())
	}
}

func DeleteShareRelatedRBAC(name string, t *testing.T) {
	t.Logf("%s: start delete share related rbac %s", time.Now().String(), name)
	err := clusterRoleBindingClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting cluster role %s: %s", name, err.Error())
	}
	err = clusterRoleClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting cluster role binding %s: %s", name, err.Error())
	}
	t.Logf("%s: completed share related rbac deletion %s", time.Now().String(), name)
}

func CreateShareRelatedRBAC(name string, t *testing.T) {
	t.Logf("%s: start create share related rbac %s", time.Now().String(), name)
	createShareClusterRole(name, t)
	createShareClusterRoleBinding(name, t)
	t.Logf("%s: completed share related rbac creation %s", time.Now().String(), name)
}
