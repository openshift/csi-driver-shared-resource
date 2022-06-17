package csidriver

import (
	"net/http"

	operatorv1 "github.com/openshift/api/operator/v1"
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
	return req.Kind.Kind == "Pod"
}

// Authorized implements Webhook interface
func (s *SharedResourcesCSIDriverWebhook) Authorized(request admissionctl.Request) admissionctl.Response {
	return s.authorizePod(request)
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
