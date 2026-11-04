package framework

import (
	"context"
	"os"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

// GetDriverServiceAccount returns the ServiceAccount name for the CSI driver.
// Defaults to shared-resource-csi-driver-node, can be overridden via CSI_DRIVER_SERVICE_ACCOUNT env var.
func GetDriverServiceAccount() string {
	if sa := os.Getenv("CSI_DRIVER_SERVICE_ACCOUNT"); sa != "" {
		return sa
	}
	return "shared-resource-csi-driver-node"
}

func createShareRole(t *TestArgs) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"use"},
				APIGroups:     []string{"sharedresource.openshift.io"},
				Resources:     []string{"sharedconfigmaps", "sharedsecrets"},
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
	subject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Namespace: t.Name,
		Name:      "default",
	}
	if t.TestGroupRBACOnly {
		subject.Kind = "Group"
		subject.Name = "system:serviceaccounts"
		subject.Namespace = ""
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Name,
		},
		Subjects: []rbacv1.Subject{
			subject,
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

// CreateDriverRBAC creates namespace-scoped RBAC that allows the CSI driver
// to access source secrets/configmaps. This is required now that the driver
// no longer has cluster-wide permissions.
// NOTE: ResourceNames are not specified because list/watch operations cannot
// be restricted to specific resource names in Kubernetes RBAC.
func CreateDriverRBAC(t *TestArgs, namespace string, resourceType string, resourceNames ...string) {
	t.T.Logf("%s: start create driver rbac in namespace %s", time.Now().String(), namespace)

	roleName := "csi-driver-" + t.Name
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{resourceType},
				// ResourceNames intentionally omitted - list/watch require unrestricted access
			},
		},
	}

	roleClient := kubeClient.RbacV1().Roles(namespace)
	_, err := roleClient.Create(context.TODO(), role, metav1.CreateOptions{})
	if err != nil {
		if kerrors.IsAlreadyExists(err) {
			// Role exists - update it to add the new resource type if not already present
			existingRole, getErr := roleClient.Get(context.TODO(), roleName, metav1.GetOptions{})
			if getErr != nil {
				t.T.Fatalf("error getting existing driver role: %s", getErr.Error())
			}
			// Check if the resource type is already in the rules
			hasResourceType := false
			for _, rule := range existingRole.Rules {
				for _, res := range rule.Resources {
					if res == resourceType {
						hasResourceType = true
						break
					}
				}
			}
			// Add the resource type if it's not already present
			if !hasResourceType {
				existingRole.Rules[0].Resources = append(existingRole.Rules[0].Resources, resourceType)
				_, updateErr := roleClient.Update(context.TODO(), existingRole, metav1.UpdateOptions{})
				if updateErr != nil {
					t.T.Fatalf("error updating driver role: %s", updateErr.Error())
				}
				t.T.Logf("Updated existing role %s to include resource type %s", roleName, resourceType)
			}
		} else {
			t.T.Fatalf("error creating driver role: %s", err.Error())
		}
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      GetDriverServiceAccount(),
				Namespace: client.DefaultNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
	}

	roleBindingClient := kubeClient.RbacV1().RoleBindings(namespace)
	_, err = roleBindingClient.Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating driver role binding: %s", err.Error())
	}

	t.T.Logf("%s: completed driver rbac creation in namespace %s", time.Now().String(), namespace)
}

// DeleteDriverRBAC deletes driver RBAC created by CreateDriverRBAC
func DeleteDriverRBAC(t *TestArgs, namespace string) {
	roleName := "csi-driver-" + t.Name
	roleBindingClient := kubeClient.RbacV1().RoleBindings(namespace)
	err := roleBindingClient.Delete(context.TODO(), roleName, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Logf("warning: error deleting driver role binding %s: %s", roleName, err.Error())
	}
	roleClient := kubeClient.RbacV1().Roles(namespace)
	err = roleClient.Delete(context.TODO(), roleName, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Logf("warning: error deleting driver role %s: %s", roleName, err.Error())
	}
}
