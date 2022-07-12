package framework

import (
	"context"
	"strings"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shareapi "github.com/openshift/api/sharedresource/v1alpha1"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
)

func CreateShare(t *TestArgs) {
	t.T.Logf("%s: start create share %s", time.Now().String(), t.Name)
	share := &shareapi.SharedConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Name,
		},
		Spec: shareapi.SharedConfigMapSpec{
			ConfigMapRef: shareapi.SharedConfigMapReference{
				Name:      "openshift-install",
				Namespace: "openshift-config",
			},
		},
	}
	_, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Create(context.TODO(), share, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test share: %s", err.Error())
	}
	t.T.Logf("%s: completed create share %s", time.Now().String(), share.Name)
	if t.SecondShare {
		t.T.Logf("%s: start create share %s", time.Now().String(), t.SecondName)
		share := &shareapi.SharedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.SecondName,
			},
			Spec: shareapi.SharedSecretSpec{
				SecretRef: shareapi.SharedSecretReference{
					Name:      "pull-secret",
					Namespace: "openshift-config",
				},
			},
		}
		_, err := shareClient.SharedresourceV1alpha1().SharedSecrets().Create(context.TODO(), share, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			t.T.Fatalf("error creating test share: %s", err.Error())
		}
		t.T.Logf("%s: completed create share %s", time.Now().String(), share.Name)
	}
}

func CreateShareSecretWithOpenshiftPrefix(t *TestArgs) {
	t.T.Logf("%s: start creating share secret in namespace %s", time.Now().String(), client.DefaultNamespace)
	share := &shareapi.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openshift-etc-pki-entitlement",
		},
		Spec: shareapi.SharedSecretSpec{
			SecretRef: shareapi.SharedSecretReference{
				Name:      "etc-pki-entitlement",
				Namespace: "openshift-config-managed",
			},
		},
	}
	_, err := shareClient.SharedresourceV1alpha1().SharedSecrets().Create(context.TODO(), share, metav1.CreateOptions{})
	if err != nil && strings.HasPrefix(share.Name, "openshift-") && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("Shared secret %s is not allowed, unless listed under OCP pre-populated shared resource: %s", share.Name, err.Error())
	} else if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test share: %s", err.Error())
	} else {
		t.T.Logf("%s: completed create share secret with prefix 'openshift-' %s", time.Now().String(), share.Name)
	}
}

func ChangeShare(t *TestArgs) {
	name := t.Name
	t.T.Logf("%s: start change share %s", time.Now().String(), name)
	share, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		t.T.Fatalf("error getting share %s: %s", name, err.Error())
	}
	share.Spec.ConfigMapRef.Name = t.ChangeName
	_, err = shareClient.SharedresourceV1alpha1().SharedConfigMaps().Update(context.TODO(), share, metav1.UpdateOptions{})
	if err != nil {
		t.T.Fatalf("error updating share %s: %s", name, err.Error())
	}
	t.T.Logf("%s: completed change share %s", time.Now().String(), name)
}

func DeleteShare(t *TestArgs) {
	name := t.Name
	if len(t.ShareToDelete) > 0 {
		name = t.ShareToDelete
	}
	t.T.Logf("%s: start delete share %s type %s", time.Now().String(), name, string(t.ShareToDeleteType))
	var err error
	switch {
	case t.ShareToDeleteType == consts.ResourceReferenceTypeSecret:
		err = shareClient.SharedresourceV1alpha1().SharedSecrets().Delete(context.TODO(), name, metav1.DeleteOptions{})
	case t.ShareToDeleteType == consts.ResourceReferenceTypeConfigMap:
		err = shareClient.SharedresourceV1alpha1().SharedConfigMaps().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", name, err.Error())
	}
	t.T.Logf("%s: completed delete share %s", time.Now().String(), name)
}
