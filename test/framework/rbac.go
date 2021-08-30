package framework

import (
	"context"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createShareClusterRole(t *TestArgs) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get", "list", "watch"},
				APIGroups:     []string{"storage.openshift.io"},
				Resources:     []string{"sharedresources"},
				ResourceNames: []string{t.Name},
			},
		},
	}
	if t.SecondShare {
		clusterRole.Rules[0].ResourceNames = append(clusterRole.Rules[0].ResourceNames, t.SecondName)
	}
	t.T.Logf("creating cluster role: %#v", clusterRole)
	_, err := clusterRoleClient.Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test cluster role: %s", err.Error())
	}
}

func createShareClusterRoleBinding(t *TestArgs) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: t.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     t.Name,
		},
	}
	t.T.Logf("creating cluster role binding: %#v", clusterRoleBinding)
	_, err := clusterRoleBindingClient.Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test cluster role binding: %s", err.Error())
	}
}

func DeleteShareRelatedRBAC(t *TestArgs) {
	t.T.Logf("%s: start delete share related rbac %s", time.Now().String(), t.Name)
	err := clusterRoleBindingClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting cluster role %s: %s", t.Name, err.Error())
	}
	err = clusterRoleClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting cluster role binding %s: %s", t.Name, err.Error())
	}
	t.T.Logf("%s: completed share related rbac deletion %s", time.Now().String(), t.Name)
}

func CreateShareRelatedRBAC(t *TestArgs) {
	t.T.Logf("%s: start create share related rbac %s", time.Now().String(), t.Name)
	createShareClusterRole(t)
	createShareClusterRoleBinding(t)
	t.T.Logf("%s: completed share related rbac creation %s", time.Now().String(), t.Name)
}
