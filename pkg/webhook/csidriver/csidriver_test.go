package csidriver

import (
	"encoding/json"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/api/sharedresource/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	podGvr             = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	sharedSecretGvr    = metav1.GroupVersionResource{Group: "", Version: "sharedresource.openshift.io/v1alpha1", Resource: "sharedsecrets"}
	sharedConfigMapGvr = metav1.GroupVersionResource{Group: "", Version: "sharedresource.openshift.io/v1alpha1", Resource: "sharedconfigmaps"}
)

func TestAuthorize(t *testing.T) {
	t.Parallel()
	truVal := true
	var Request v1.AdmissionRequest
	testCases := []struct {
		name            string
		shouldAdmit     bool
		msg             string
		operation       v1.Operation
		pod             *corev1.Pod
		sharedSecret    *v1alpha1.SharedSecret
		sharedConfigMap *v1alpha1.SharedConfigMap
		kind            string
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
			kind: "Pod",
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
			kind: "Pod",
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
			kind: "Pod",
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
			kind: "Pod",
		},
		{
			name:        "Create shared secret with prefix `openshift-`",
			shouldAdmit: true,
			operation:   v1.Create,
			sharedSecret: &v1alpha1.SharedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift-etc-pki-entitlement",
				},
				Spec: v1alpha1.SharedSecretSpec{
					SecretRef: v1alpha1.SharedSecretReference{
						Name:      "etc-pki-entitlement",
						Namespace: "openshift-config-managed",
					},
				},
			},
			kind: "SharedSecret",
		},
		{
			name:        "Create shared secret without prefix `openshift-`",
			shouldAdmit: true,
			operation:   v1.Create,
			sharedSecret: &v1alpha1.SharedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share-secret1",
				},
				Spec: v1alpha1.SharedSecretSpec{
					SecretRef: v1alpha1.SharedSecretReference{
						Name:      "secret1",
						Namespace: "test",
					},
				},
			},
			kind: "SharedSecret",
		},
		{
			name:        "Create shared secret with prefix `openshift-` if not available in ocp-sharedsecret-list",
			shouldAdmit: false,
			operation:   v1.Create,
			sharedSecret: &v1alpha1.SharedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift-test-secret",
				},
				Spec: v1alpha1.SharedSecretSpec{
					SecretRef: v1alpha1.SharedSecretReference{
						Name:      "test-secret",
						Namespace: "test",
					},
				},
			},
			kind: "SharedSecret",
		},
		{
			name:        "Create shared configmap without prefix `openshift-`",
			shouldAdmit: true,
			operation:   v1.Create,
			sharedConfigMap: &v1alpha1.SharedConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "share-configmap1",
				},
				Spec: v1alpha1.SharedConfigMapSpec{
					ConfigMapRef: v1alpha1.SharedConfigMapReference{
						Name:      "configmap1",
						Namespace: "test",
					},
				},
			},
			kind: "SharedConfigMap",
		},
		{
			name:        "Create shared configmap with prefix `openshift-` if not available in ocp-sharedconfigmap-list",
			shouldAdmit: false,
			operation:   v1.Create,
			sharedConfigMap: &v1alpha1.SharedConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift-test-shared-config",
				},
				Spec: v1alpha1.SharedConfigMapSpec{
					ConfigMapRef: v1alpha1.SharedConfigMapReference{
						Name:      "test-shared-config",
						Namespace: "test",
					},
				},
			},
			kind: "SharedConfigMap",
		},
	}

	for _, tc := range testCases {
		switch tc.kind {
		case "Pod":
			pod := tc.pod
			raw, err := json.Marshal(pod)
			if err != nil {
				t.Fatal(err)
			}

			Request = admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: raw,
				},
				Kind: metav1.GroupVersionKind{
					Kind: tc.kind,
				},
				Resource:  podGvr,
				Operation: tc.operation,
			}
		case "SharedConfigMap":
			configmap := tc.sharedConfigMap
			raw, err := json.Marshal(configmap)
			if err != nil {
				t.Fatal(err)
			}

			Request = admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: raw,
				},
				Kind: metav1.GroupVersionKind{
					Kind: tc.kind,
				},
				Resource:  sharedConfigMapGvr,
				Operation: tc.operation,
			}
		case "SharedSecret":
			secret := tc.sharedSecret
			raw, err := json.Marshal(secret)
			if err != nil {
				t.Fatal(err)
			}

			Request = admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: raw,
				},
				Kind: metav1.GroupVersionKind{
					Kind: tc.kind,
				},
				Resource:  sharedSecretGvr,
				Operation: tc.operation,
			}
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
