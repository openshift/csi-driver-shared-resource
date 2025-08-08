package framework

import (
	"context"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shareapi "github.com/openshift/api/sharedresource/v1alpha1"

	"github.com/openshift/csi-driver-shared-resource/pkg/consts"
)

func CreateShare(t *TestArgs) {
	t.T.Logf("%s: start create share %s", time.Now().String(), t.Name)
	shareName := t.Name
	if len(t.ShareNameOverride) > 0 {
		shareName = t.ShareNameOverride
	}
	share := &shareapi.SharedConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: shareName,
		},
		Spec: shareapi.SharedConfigMapSpec{
			ConfigMapRef: shareapi.SharedConfigMapReference{
				Name:      "openshift-install",
				Namespace: "openshift-config",
			},
		},
	}
	_, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Create(context.TODO(), share, metav1.CreateOptions{})
	if err == nil && t.TestShareCreateRejected {
		dumpCSIPods(t)
		time.Sleep(5 * time.Second)
		t.T.Fatalf("share %s creation incorrectly allowed", shareName)
	}
	if err != nil && t.TestShareCreateRejected {
		t.T.Logf("TestShareCreateRejected got error %s on creation of %s", err.Error(), shareName)
		return
	}
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

func CreateReservedOpenShiftShare(t *TestArgs) {
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
	if err != nil && !kerrors.IsAlreadyExists(err) {
		dumpCSIPods(t)
		time.Sleep(5 * time.Second)
		t.T.Fatalf("share %s creation incorrectly prevented: %s", share.Name, err.Error())
	}

}

func ChangeShare(t *TestArgs) {
	name := t.Name
	if len(t.ShareNameOverride) > 0 {
		name = t.ShareNameOverride
	}
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
	if len(t.ShareNameOverride) > 0 {
		name = t.ShareNameOverride
	}
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

func CreateTestSharedSecret(t *TestArgs, shareName, secretName, secretNamespace string) error {
	share := &shareapi.SharedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: shareName,
		},
		Spec: shareapi.SharedSecretSpec{
			SecretRef: shareapi.SharedSecretReference{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		},
	}

	t.T.Logf("Attempting to create SharedSecret %s...", share.Name)
	_, err := shareClient.SharedresourceV1alpha1().SharedSecrets().Create(context.TODO(), share, metav1.CreateOptions{})

	// Logging immediate result of the Create call, even if err is nil
	t.T.Logf("API call to Create SharedSecret %s returned error: %v", share.Name, err)

	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}

	// trying to get the resource right after creating it.
	t.T.Logf("Immediately trying to GET SharedSecret %s after creation...", share.Name)
	createdShare, getErr := shareClient.SharedresourceV1alpha1().SharedSecrets().Get(context.TODO(), share.Name, metav1.GetOptions{})
	t.T.Logf("Immediate GET for SharedSecret %s returned error: %v", share.Name, getErr)
	if getErr == nil {
		t.T.Logf("Successfully got SharedSecret %s immediately after creation with UID: %s", createdShare.Name, createdShare.UID)
	}

	return nil
}

func CreateTestSharedConfigMap(t *TestArgs, shareName, cmName, cmNamespace string) error {
	share := &shareapi.SharedConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: shareName,
		},
		Spec: shareapi.SharedConfigMapSpec{
			ConfigMapRef: shareapi.SharedConfigMapReference{
				Name:      cmName,
				Namespace: cmNamespace,
			},
		},
	}

	t.T.Logf("Attempting to create SharedConfigMap %s...", share.Name)
	_, err := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Create(context.TODO(), share, metav1.CreateOptions{})

	// Logging immediate result of the Create call, even if err is nil
	t.T.Logf("API call to Create SharedConfigMap %s returned error: %v", share.Name, err)

	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}

	// trying to GET the resource right after creating it.
	t.T.Logf("Immediately trying to GET SharedConfigMap %s after creation...", share.Name)
	createdShare, getErr := shareClient.SharedresourceV1alpha1().SharedConfigMaps().Get(context.TODO(), share.Name, metav1.GetOptions{})
	t.T.Logf("Immediate GET for SharedConfigMap %s returned error: %v", share.Name, getErr)
	if getErr == nil {
		t.T.Logf("Successfully got SharedConfigMap %s immediately after creation with UID: %s", createdShare.Name, createdShare.UID)
	}

	return nil
}
