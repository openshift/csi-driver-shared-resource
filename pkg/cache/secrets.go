package cache

import (
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	sharev1alpha1 "github.com/openshift/csi-driver-shared-resource/pkg/api/sharedresource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

/*
Some old fashioned comments that describe what we are doing in this golang file.

First, some notes on cardinality:
- 1 share references 1 and only 1 secret currently
- given share related cardinality, many pods can reference a given secret via share CSI Volumes

Second, events.  We process Add and Update secret events from the controller in the same way, so we have an UpsertSecret function.
For delete events, the DelSecret is called.

On the use of sync.Map, see the comments in share.go

*/

var (
	// secrets is our global configmap id (namespace + name) to secret map, where entries are populated from
	// controller events; it serves to facilitate quick lookup during share event processing, when the share references
	// a secret
	secrets = sync.Map{}
	// secretUpsertCallbacks has a key of the CSI volume ID and a value of the function to be called when a given
	// secret is updated, assuming the driver has mounted a share CSI volume with the configmap in a pod somewhere, and
	// the corresponding storage on the pod gets updated by the function that is the value of the entry.  Otherwise,
	// this map is empty and configmap updates result in a no-op.  This map is used both when we get an event for a given
	// secret or a series of events as a result of a relist from the controller.
	secretUpsertCallbacks = sync.Map{}
	// same thing as secretUpsertCallbacks but deletion of secrets, and of of course the controller relist does not
	// come into play here.
	secretDeleteCallbacks = sync.Map{}
	// secretsWithShare is a filtered list of secrets where, via share events, we know at least one active share references
	// a given secret; when possible we range over this list vs. secrets
	secretsWithShare = sync.Map{}
	// sharesWaitingOnSecrets conversely is for when a share has been created that references a secret, but that
	// secret has not been recognized by the controller; quite possibly timing events on when we learn of sharedSecrets
	// and secret if they happen to be created at roughly the same time come into play; also, if a pod with a share
	// pointing to a secret has been provisioned, but the the csi driver daemonset has been restarted, such timing
	// of events where we learn of sharedSecrets before their secrets can also occur, as we attempt to rebuild the CSI driver's
	// state
	sharesWaitingOnSecrets = sync.Map{}
)

// GetSecret retrieves a secret from the list of secrets referenced by SharedSecrets
func GetSecret(key interface{}) *corev1.Secret {
	obj, loaded := secretsWithShare.Load(key)
	if loaded {
		s, _ := obj.(*corev1.Secret)
		return s
	}
	return nil
}

// SetSecret based on the shared-data-key, which contains the resource's namespace and name, this
// method can fetch and store it on cache.  This method is called when the controller is not watching
// secrets, and the CSI driver must retrieve the secret when processing a NodePublishVolume call
// from the kubelet.
func SetSecret(kubeClient kubernetes.Interface, sharedDataKey string) error {
	ns, name, err := SplitKey(sharedDataKey)
	if err != nil {
		return err
	}

	secret, err := kubeClient.CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	UpsertSecret(secret)
	return nil
}

// UpsertSecret adds or updates as needed the secret to our various maps for correlating with SharedSecrets and
// calls registered upsert callbacks
func UpsertSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	klog.V(6).Infof("UpsertSecret key %s", key)
	secrets.Store(key, secret)
	// in case share arrived before secret
	processedSharesWithoutSecrets := []string{}
	sharesWaitingOnSecrets.Range(func(key, value interface{}) bool {
		shareKey := key.(string)
		share := value.(*sharev1alpha1.SharedSecret)
		br := share.Spec.Secret
		secretKey := BuildKey(br)
		secretsWithShare.Store(secretKey, secret)
		//NOTE: share update ranger will store share in sharedSecrets sync.Map
		// and we are supplying only this specific share to the csi driver update range callbacks.
		shareSecretsUpdateCallbacks.Range(buildRanger(buildCallbackMap(share.Name, share)))
		processedSharesWithoutSecrets = append(processedSharesWithoutSecrets, shareKey)
		return true
	})
	for _, shareKey := range processedSharesWithoutSecrets {
		sharesWaitingOnSecrets.Delete(shareKey)
	}
	// otherwise process any share that arrived after the secret
	secretsWithShare.Store(key, secret)
	secretUpsertCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
}

// DelSecret deletes this secret from the various secret related maps
func DelSecret(secret *corev1.Secret) {
	key := GetKey(secret)
	klog.V(4).Infof("DelSecret key %s", key)
	secrets.Delete(key)
	secretDeleteCallbacks.Range(buildRanger(buildCallbackMap(key, secret)))
	secretsWithShare.Delete(key)
}

// RegisterSecretUpsertCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// if the corresponding share references a secret, then the function registered here will be called to possibly change
// storage
func RegisterSecretUpsertCallback(volID string, f func(key, value interface{}) bool) {
	secretUpsertCallbacks.Store(volID, f)
	// cycle through the secrets with sharedSecrets, where if the share associated with the volID CSI volume mount references
	// one of the secrets provided by the Range, the storage of the corresponding data on the pod will be completed using
	// the supplied function
	secretsWithShare.Range(f)
}

// UnregisterSecretUpsertCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSecretUpsertCallback(volID string) {
	secretUpsertCallbacks.Delete(volID)
}

// RegisterSecretDeleteCallback will be called as part of the kubelet sending a mount CSI volume request for a pod;
// it records the CSI driver function to be called when a secret is deleted, so that the CSI
// driver can remove any storage mounted in the pod for the given secret
func RegisterSecretDeleteCallback(volID string, f func(key, value interface{}) bool) {
	secretDeleteCallbacks.Store(volID, f)
}

// UnregisterSecretDeleteCallback will be called as part of the kubelet sending a delete CSI volume request for a pod
// that is going away, and we remove the corresponding function for that volID
func UnregisterSecretDeleteCallback(volID string) {
	secretDeleteCallbacks.Delete(volID)
}
