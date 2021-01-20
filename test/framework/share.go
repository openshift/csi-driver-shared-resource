package framework

import (
	"context"
	"testing"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shareapi "github.com/openshift/csi-driver-projected-resource/pkg/api/projectedresource/v1alpha1"
)

func CreateShare(name string, t *testing.T) {
	t.Logf("%s: start create share %s", time.Now().String(), name)
	share := &shareapi.Share{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
	_, err := shareClient.ProjectedresourceV1alpha1().Shares().Create(context.TODO(), share, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.Fatalf("error creating test share: %s", err.Error())
	}
	t.Logf("%s: completed create share %s", time.Now().String(), name)
}

func ChangeShare(name string, t *testing.T) {
	t.Logf("%s: start change share %s", time.Now().String(), name)
	share, err := shareClient.ProjectedresourceV1alpha1().Shares().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting share %s: %s", name, err.Error())
	}
	share.Spec.BackingResource.Kind = "Secret"
	share.Spec.BackingResource.Name = "pull-secret"
	_, err = shareClient.ProjectedresourceV1alpha1().Shares().Update(context.TODO(), share, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("error updating share %s: %s", name, err.Error())
	}
	t.Logf("%s: completed change share %s", time.Now().String(), name)
}

func DeleteShare(name string, t *testing.T) {
	t.Logf("%s: start delete share %s", time.Now().String(), name)
	err := shareClient.ProjectedresourceV1alpha1().Shares().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		t.Fatalf("error deleting share %s: %s", name, err.Error())
	}
	t.Logf("%s: completed delete share %s", time.Now().String(), name)
}
