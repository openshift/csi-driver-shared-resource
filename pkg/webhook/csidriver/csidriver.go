package csidriver

import (
	"fmt"
	"net/http"

	operatorv1 "github.com/openshift/api/operator/v1"
	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	"github.com/openshift/csi-driver-shared-resource/pkg/config"

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
	rn *config.ReservedNames
}

// NewWebhook creates a new webhook
func NewWebhook(rn *config.ReservedNames) *SharedResourcesCSIDriverWebhook {
	return &SharedResourcesCSIDriverWebhook{rn: rn}
}

// GetURI implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) GetURI() string { return URI }

// Name implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) Name() string { return WebhookName }

// Validate if the incoming request even valid
func (s *SharedResourcesCSIDriverWebhook) Validate(req admissionctl.Request) bool {
	return req.Kind.Kind == "Pod" || req.Kind.Kind == "SharedSecret" || req.Kind.Kind == "SharedConfigMap"
}

// Authorized implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) Authorized(request admissionctl.Request) admissionctl.Response {
	pod, perr := s.renderPod(request)
	ss, sserr := s.renderSharedSecret(request)
	sc, scerr := s.renderSharedConfigMap(request)

	if perr != nil && sserr != nil && scerr != nil {
		klog.Error(perr, "Couldn't render a Pod from the incoming request")
		klog.Error(sserr, "Couldn't render a SharedSecret from the incoming request")
		klog.Error(scerr, "Couldn't render a SharedConfigMap from the incoming request")
		return admissionctl.Errored(http.StatusBadRequest, perr)
	}

	switch {
	case pod != nil && perr == nil:
		return s.authorizePod(request, pod)
	case ss != nil && sserr == nil:
		return s.authorizeSharedSecret(request, ss)
	case sc != nil && scerr == nil:
		return s.authorizeSharedConfigMap(request, sc)
	}
	ret := admissionctl.Allowed("type we are unconcerned with")
	ret.UID = request.AdmissionRequest.UID
	return ret
}

func (s *SharedResourcesCSIDriverWebhook) authorizePod(request admissionctl.Request, pod *corev1.Pod) admissionctl.Response {
	klog.V(2).Info("admitting pod with SharedResourceCSIVolume")
	var ret admissionctl.Response

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

func (s *SharedResourcesCSIDriverWebhook) authorizeSharedSecret(request admissionctl.Request, ss *sharev1alpha1.SharedSecret) admissionctl.Response {
	klog.V(2).Info("admitting shared secret with SharedResourceCSIVolume")
	var ret admissionctl.Response

	if s.rn.ValidateSharedSecretOpenShiftName(ss.Name, ss.Spec.SecretRef.Namespace, ss.Spec.SecretRef.Name) {
		ret = admissionctl.Allowed("Allowed to create SharedSecret")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}
	ret = admissionctl.Denied(fmt.Sprintf("Not allowed to create SharedSecret with name %s as it violates the reserved names list", ss.Name))
	ret.UID = request.AdmissionRequest.UID
	return ret
}

func (s *SharedResourcesCSIDriverWebhook) authorizeSharedConfigMap(request admissionctl.Request, scm *sharev1alpha1.SharedConfigMap) admissionctl.Response {
	klog.V(2).Info("admitting shared configmap with SharedResourceCSIVolume")
	var ret admissionctl.Response

	if s.rn.ValidateSharedConfigMapOpenShiftName(scm.Name, scm.Spec.ConfigMapRef.Namespace, scm.Spec.ConfigMapRef.Name) {
		ret = admissionctl.Allowed("Allowed to create SharedConfigMap")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}
	ret = admissionctl.Denied(fmt.Sprintf("Not allowed to create SharedConfigMap with name %s as it violates the reserved names list", scm.Name))
	ret.UID = request.AdmissionRequest.UID
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

func (s *SharedResourcesCSIDriverWebhook) renderSharedSecret(request admissionctl.Request) (*sharev1alpha1.SharedSecret, error) {
	decoder, err := admissionctl.NewDecoder(scheme)
	if err != nil {
		return nil, err
	}
	sharedSecret := &sharev1alpha1.SharedSecret{}
	if len(request.OldObject.Raw) > 0 {
		err = decoder.DecodeRaw(request.OldObject, sharedSecret)
	} else {
		err = decoder.DecodeRaw(request.Object, sharedSecret)
	}

	return sharedSecret, err
}

func (s *SharedResourcesCSIDriverWebhook) renderSharedConfigMap(request admissionctl.Request) (*sharev1alpha1.SharedConfigMap, error) {
	decoder, err := admissionctl.NewDecoder(scheme)
	if err != nil {
		return nil, err
	}
	sharedConfigMap := &sharev1alpha1.SharedConfigMap{}
	if len(request.OldObject.Raw) > 0 {
		err = decoder.DecodeRaw(request.OldObject, sharedConfigMap)
	} else {
		err = decoder.DecodeRaw(request.Object, sharedConfigMap)
	}

	return sharedConfigMap, err
}
