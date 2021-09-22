package framework

import (
	"context"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createShareRole(t *TestArgs) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"use"},
				APIGroups:     []string{"storage.openshift.io"},
				Resources:     []string{"sharedresources"},
				ResourceNames: []string{t.Name},
			},
		},
	}
	if t.SecondShare {
		role.Rules[0].ResourceNames = append(role.Rules[0].ResourceNames, t.SecondName)
	}

	roleClient := kubeClient.RbacV1().Roles(t.Name)

	_, err := roleClient.Create(context.TODO(), role, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test cluster role: %s", err.Error())
	}
}

func createShareRoleBinding(t *TestArgs) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Name,
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
			Kind:     "Role",
			Name:     t.Name,
		},
	}

	roleBindingClient := kubeClient.RbacV1().RoleBindings(t.Name)

	_, err := roleBindingClient.Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test cluster role binding: %s", err.Error())
	}
}

func DeleteShareRelatedRBAC(t *TestArgs) {
	t.T.Logf("%s: start delete share related rbac %s", time.Now().String(), t.Name)
	roleBindingClient := kubeClient.RbacV1().RoleBindings(t.Name)
	err := roleBindingClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting cluster role %s: %s", t.Name, err.Error())
	}
	roleClient := kubeClient.RbacV1().Roles(t.Name)
	err = roleClient.Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting cluster role binding %s: %s", t.Name, err.Error())
	}
	t.T.Logf("%s: completed share related rbac deletion %s", time.Now().String(), t.Name)
}

func CreateShareRelatedRBAC(t *TestArgs) {
	t.T.Logf("%s: start create share related rbac %s", time.Now().String(), t.Name)
	createShareRole(t)
	createShareRoleBinding(t)
	t.T.Logf("%s: completed share related rbac creation %s", time.Now().String(), t.Name)
}
