package csidriver

import (
	"encoding/json"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	podGvr = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
)

func TestAuthorize(t *testing.T) {
	t.Parallel()
	truVal := true
	testCases := []struct {
		name        string
		shouldAdmit bool
		msg         string
		operation   v1.Operation
		pod         *corev1.Pod
	}{
		{
			name:        "Create pod without volumes",
			shouldAdmit: true,
			operation:   v1.Create,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{},
			},
		},
		{
			name:        "Create pod with a volume, the volume is not a SharedResourceCsiDriver volume",
			shouldAdmit: true,
			operation:   v1.Create,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-2",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "emptydir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
		{
			name:        "Create pod with a SharedResourceCSI volume with ReadOnly not set",
			shouldAdmit: false,
			operation:   v1.Create,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-3",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "csi-one",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:           string(operatorv1.SharedResourcesCSIDriver),
									VolumeAttributes: map[string]string{"sharedConfigMap": "shared-cm-test"},
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "Create pod with a SharedResourceCSI volume with ReadOnly true",
			shouldAdmit: true,
			operation:   v1.Create,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-4",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "csi-one",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									ReadOnly:         &truVal,
									Driver:           string(operatorv1.SharedResourcesCSIDriver),
									VolumeAttributes: map[string]string{"sharedConfigMap": "shared-cm-test"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		pod := tc.pod
		raw, err := json.Marshal(pod)
		if err != nil {
			t.Fatal(err)
		}

		Request := admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: raw,
			},
			Resource:  podGvr,
			Operation: tc.operation,
		}

		hook := NewWebhook()
		req := admissionctl.Request{
			AdmissionRequest: Request,
		}

		response := hook.Authorized(req)

		if response.Allowed != tc.shouldAdmit {
			t.Fatalf("Mismatch: %s Should admit %t. got %t", tc.name, tc.shouldAdmit, response.Allowed)
		}
	}
}
