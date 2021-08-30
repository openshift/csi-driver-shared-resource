package cache

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	storagev1alpha1 "github.com/openshift/api/storage/v1alpha1"
)

func TestGetNamespaceAndNameFrom(t *testing.T) {
	type args struct {
		ref storagev1alpha1.ResourceReference
	}
	tests := []struct {
		name          string
		args          args
		wantNamespace string
		wantName      string
	}{
		{
			name: "ConfigMap Resource Reference",
			args: args{
				ref: storagev1alpha1.ResourceReference{
					Type: storagev1alpha1.ResourceReferenceTypeConfigMap,
					ConfigMap: &storagev1alpha1.ResourceReferenceConfigMap{
						Name:      "my-configmap",
						Namespace: "my-namespace",
					},
				},
			},
			wantName:      "my-configmap",
			wantNamespace: "my-namespace",
		},
		{
			name: "Secret Resource Reference",
			args: args{
				ref: storagev1alpha1.ResourceReference{
					Type: storagev1alpha1.ResourceReferenceTypeSecret,
					Secret: &storagev1alpha1.ResourceReferenceSecret{
						Name:      "my-secret",
						Namespace: "my-namespace",
					},
				},
			},
			wantName:      "my-secret",
			wantNamespace: "my-namespace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetNamespaceAndNameFrom(tt.args.ref)
			if got != tt.wantNamespace {
				t.Errorf("GetNamespaceAndNameFrom() got = %v, want %v", got, tt.wantNamespace)
			}
			if got1 != tt.wantName {
				t.Errorf("GetNamespaceAndNameFrom() got1 = %v, want %v", got1, tt.wantName)
			}
		})
	}
}

func TestGetKey(t *testing.T) {
	type args struct {
		o interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Kubernetes ConfigMap",
			args: args{
				o: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-configmap",
						Namespace: "my-namespace",
					},
				},
			},
			want: "my-namespace:my-configmap",
		},
		{
			name: "Kubernetes Secret",
			args: args{
				o: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "my-namespace",
					},
				},
			},
			want: "my-namespace:my-secret",
		},
		{
			name: "ConfigMap Resource Reference",
			args: args{
				o: storagev1alpha1.ResourceReference{
					Type: storagev1alpha1.ResourceReferenceTypeConfigMap,
					ConfigMap: &storagev1alpha1.ResourceReferenceConfigMap{
						Name:      "my-configmap",
						Namespace: "my-namespace",
					},
				},
			},
			want: "my-namespace:my-configmap",
		},
		{
			name: "Secret Resource Reference",
			args: args{
				o: storagev1alpha1.ResourceReference{
					Type: storagev1alpha1.ResourceReferenceTypeSecret,
					Secret: &storagev1alpha1.ResourceReferenceSecret{
						Name:      "my-secret",
						Namespace: "my-namespace",
					},
				},
			},
			want: "my-namespace:my-secret",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetKeyFrom(tt.args.o); got != tt.want {
				t.Errorf("GetKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildKey(t *testing.T) {
	type args struct {
		namespace string
		name      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "generic namespace and name",
			args: args{
				namespace: "my-namespace",
				name:      "my-name",
			},
			want: "my-namespace:my-name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildKeyUsing(tt.args.namespace, tt.args.name); got != tt.want {
				t.Errorf("BuildKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
