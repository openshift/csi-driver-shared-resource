package csidriver

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

type Payload struct {
	StringData map[string]string
	ByteData   map[string][]byte
}

func ProcessFileSystemError(obj runtime.Object, err error) {
	msg := fmt.Sprintf("%s", err.Error())
	klog.Errorf("%s", msg)
	client.GetRecorder().Eventf(obj, corev1.EventTypeWarning, "FileSystemError", msg)

}
