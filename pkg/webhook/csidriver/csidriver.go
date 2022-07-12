package csidriver

import (
	"fmt"
	"net/http"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/api/sharedresource/v1alpha1"
	"github.com/openshift/csi-driver-shared-resource/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// VolumeSourceType represents a volume source type
type VolumeSourceType string

const (
	URI                 string           = "/resource-validation"
	WebhookName         string           = "sharedresourcecsidriver"
	VolumeSourceTypeCSI VolumeSourceType = "CSI"
)

// Webhook interface
type Webhook interface {
	// Authorized will determine if the request is allowed
	Authorized(request admissionctl.Request) admissionctl.Response
	// GetURI returns the URI for the webhook
	GetURI() string
	// Validate will validate the incoming request
	Validate(admissionctl.Request) bool
	// Name is the name of the webhook
	Name() string
}

// SharedResourcesCSIDriverWebhook validates a Shared Resources CSI Driver change
type SharedResourcesCSIDriverWebhook struct {
}

// NewWebhook creates a new webhook
func NewWebhook() *SharedResourcesCSIDriverWebhook {
	return &SharedResourcesCSIDriverWebhook{}
}

// GetURI implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) GetURI() string { return URI }

// Name implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) Name() string { return WebhookName }

// Validate if the incoming request even valid
func (s *SharedResourcesCSIDriverWebhook) Validate(req admissionctl.Request) bool {
	switch req.Kind.Kind {
	case "Pod":
		return req.Kind.Kind == "Pod"
	case "SharedConfigMap":
		return req.Kind.Kind == "SharedConfigMap"
	case "SharedSecret":
		return req.Kind.Kind == "SharedSecret"
	default:
		return false
	}
}

// Authorized implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) Authorized(request admissionctl.Request) admissionctl.Response {
	switch request.Kind.Kind {
	case "Pod":
		return s.authorizePod(request)
	case "SharedConfigMap":
		return s.authorizeSharedConfigMap(request)
	case "SharedSecret":
		return s.authorizeSharedSecret(request)
	default:
		return admissionctl.Denied(fmt.Sprintf("Requesting resource kind %q is not supported", request.Kind.Kind))
	}
}

func (s *SharedResourcesCSIDriverWebhook) authorizePod(request admissionctl.Request) admissionctl.Response {
	klog.V(2).Info("admitting pod with SharedResourceCSIVolume")
	var ret admissionctl.Response

	pod, err := s.renderPod(request)
	if err != nil {
		klog.Error(err, "Couldn't render a Pod from the incoming request")
		return admissionctl.Errored(http.StatusBadRequest, err)
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.VolumeSource.CSI != nil &&
			volume.VolumeSource.CSI.Driver == string(operatorv1.SharedResourcesCSIDriver) {
			if volume.VolumeSource.CSI.ReadOnly == nil || !*volume.VolumeSource.CSI.ReadOnly {
				ret = admissionctl.Denied("Not allowed to schedule a pod with ReadOnly false SharedResourceCSIVolume")
				ret.UID = request.AdmissionRequest.UID
				return ret
			}
		}
	}
	// Hereafter, all requests are controlled
	ret = admissionctl.Allowed("Allowed to create Pod")
	ret.UID = request.AdmissionRequest.UID
	return ret
}

func (s *SharedResourcesCSIDriverWebhook) authorizeSharedSecret(request admissionctl.Request) admissionctl.Response {
	klog.V(2).Info("admitting shared secret with SharedResourceCSIDiver")

	SharedSecret, err := s.renderSharedSecret(request)
	if err != nil {
		klog.Error(err, "Couldn't render a shared secret from the incoming request")
		return admissionctl.Errored(http.StatusBadRequest, err)
	}

	return s.authorizeResource(request, SharedSecret.ObjectMeta.Name, SharedSecret.Spec.SecretRef.Name, SharedSecret.Spec.SecretRef.Namespace)
}

func (s *SharedResourcesCSIDriverWebhook) authorizeSharedConfigMap(request admissionctl.Request) admissionctl.Response {
	klog.V(2).Info("admitting shared configmap with SharedResourceCSIDiver")
	SharedConfigMap, err := s.renderSharedConfigMap(request)
	if err != nil {
		klog.Error(err, "Couldn't render a shared configmap from the incoming request")
		return admissionctl.Errored(http.StatusBadRequest, err)
	}

	return s.authorizeResource(request, SharedConfigMap.ObjectMeta.Name, SharedConfigMap.Spec.ConfigMapRef.Name, SharedConfigMap.Spec.ConfigMapRef.Namespace)
}

// authorizeResource checks whether the shared resources can be created
// with "openshift-" prefix or not.
func (s *SharedResourcesCSIDriverWebhook) authorizeResource(request admissionctl.Request, resource, resourceRefKeyNm, resourceRefKeyNs string) admissionctl.Response {

	var ret admissionctl.Response

	ret.UID = request.AdmissionRequest.UID
	if !util.IsAuthorizedSharedResource(request.Kind.Kind, resource, resourceRefKeyNm, resourceRefKeyNs) {
		ret = admissionctl.Denied("Shared resource with prefix 'openshift-' is not allowed, unless listed under OCP pre-populated shared resource")
		return ret
	}

	// Hereafter, all requests are controlled
	ret = admissionctl.Allowed("Allowed to create shared configmap")
	return ret
}

// renderPod decodes an *corev1.Pod from the incoming request.
// If the request includes an OldObject (from an update or deletion), it will be
// preferred, otherwise, the Object will be preferred.
func (s *SharedResourcesCSIDriverWebhook) renderPod(request admissionctl.Request) (*corev1.Pod, error) {
	decoder, err := admissionctl.NewDecoder(scheme)
	if err != nil {
		return nil, err
	}
	pod := &corev1.Pod{}
	if len(request.OldObject.Raw) > 0 {
		err = decoder.DecodeRaw(request.OldObject, pod)
	} else {
		err = decoder.DecodeRaw(request.Object, pod)
	}

	return pod, err
}

// renderSharedConfigMap decodes an *v1alpha1.SharedConfigMap from the incoming request.
// If the request includes an OldObject (from an update or deletion), it will be
// preferred, otherwise, the Object will be preferred.
func (s *SharedResourcesCSIDriverWebhook) renderSharedConfigMap(request admissionctl.Request) (*v1alpha1.SharedConfigMap, error) {
	decoder, err := admissionctl.NewDecoder(scheme)
	if err != nil {
		return nil, err
	}

	sharedConfigMap := &v1alpha1.SharedConfigMap{}
	if len(request.OldObject.Raw) > 0 {
		err = decoder.DecodeRaw(request.OldObject, sharedConfigMap)
	} else {
		err = decoder.DecodeRaw(request.Object, sharedConfigMap)
	}

	return sharedConfigMap, err
}

// renderSharedSecret decodes an *v1alpha1.SharedSecret from the incoming request.
// If the request includes an OldObject (from an update or deletion), it will be
// preferred, otherwise, the Object will be preferred.
func (s *SharedResourcesCSIDriverWebhook) renderSharedSecret(request admissionctl.Request) (*v1alpha1.SharedSecret, error) {
	decoder, err := admissionctl.NewDecoder(scheme)
	if err != nil {
		return nil, err
	}

	sharedSecret := &v1alpha1.SharedSecret{}
	if len(request.OldObject.Raw) > 0 {
		err = decoder.DecodeRaw(request.OldObject, sharedSecret)
	} else {
		err = decoder.DecodeRaw(request.Object, sharedSecret)
	}

	return sharedSecret, err
}
