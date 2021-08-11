package framework

import (
	"context"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shareapi "github.com/openshift/csi-driver-shared-resource/pkg/api/sharedresource/v1alpha1"
)

func CreateShare(t *TestArgs) {
	t.T.Logf("%s: start create share %s", time.Now().String(), t.Name)
	share := &shareapi.Share{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Name,
		},
		Spec: shareapi.ShareSpec{
			BackingResource: shareapi.BackingResource{
				Kind:       "ConfigMap",
				APIVersion: "v1",
				Name:       "openshift-install",
				Namespace:  "openshift-config",
			},
		},
	}
	_, err := shareClient.SharedresourceV1alpha1().Shares().Create(context.TODO(), share, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.T.Fatalf("error creating test share: %s", err.Error())
	}
	t.T.Logf("%s: completed create share %s", time.Now().String(), share.Name)
	if t.SecondShare {
		t.T.Logf("%s: start create share %s", time.Now().String(), t.SecondName)
		share := &shareapi.Share{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.SecondName,
			},
			Spec: shareapi.ShareSpec{
				BackingResource: shareapi.BackingResource{
					Kind:       "Secret",
					APIVersion: "v1",
					Name:       "pull-secret",
					Namespace:  "openshift-config",
				},
			},
		}
		_, err := shareClient.SharedresourceV1alpha1().Shares().Create(context.TODO(), share, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			t.T.Fatalf("error creating test share: %s", err.Error())
		}
		t.T.Logf("%s: completed create share %s", time.Now().String(), share.Name)
	}
}

func ChangeShare(t *TestArgs) {
	name := t.Name
	if t.SecondShare {
		name = t.SecondName
	}
	t.T.Logf("%s: start change share %s", time.Now().String(), name)
	share, err := shareClient.SharedresourceV1alpha1().Shares().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		t.T.Fatalf("error getting share %s: %s", name, err.Error())
	}
	share.Spec.BackingResource.Kind = "Secret"
	share.Spec.BackingResource.Name = "pull-secret"
	_, err = shareClient.SharedresourceV1alpha1().Shares().Update(context.TODO(), share, metav1.UpdateOptions{})
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
	t.T.Logf("%s: start delete share %s", time.Now().String(), name)
	err := shareClient.SharedresourceV1alpha1().Shares().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.T.Fatalf("error deleting share %s: %s", name, err.Error())
	}
	t.T.Logf("%s: completed delete share %s", time.Now().String(), name)
}
