package framework

import (
	"context"
	"fmt"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/csi-driver-projected-resource/pkg/client"
)

func WaitForDaemonSet(t *TestArgs) error {
	dsClient := kubeClient.AppsV1().DaemonSets(client.DefaultNamespace)
	err := wait.PollImmediate(1*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := dsClient.Get(context.TODO(), "csi-hostpathplugin", metav1.GetOptions{})
		if err != nil {
			t.T.Logf("%s: error waiting for driver daemonset to exist: %v", time.Now().String(), err)
			return false, nil
		}
		t.T.Logf("%s: found operator deployment", time.Now().String())
		return true, nil
	})
	podClient = kubeClient.CoreV1().Pods(client.DefaultNamespace)
	err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.T.Logf("%s: error listing pods: %s\n", time.Now().String(), err.Error())
			return false, nil
		}

		if t.DaemonSetUp {
			if podList.Items == nil || len(podList.Items) != 3 {
				t.T.Logf("%s: number of pods not yet at 3", time.Now().String())
				return false, nil
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
					t.T.Logf("%s: pod %s in phase %s with deletion timestamp %v\n", time.Now().String(), pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
					return false, nil
				}
			}
			t.T.Logf("%s: all 3 daemonset pods are running", time.Now().String())
		} else {
			if podList.Items == nil || len(podList.Items) == 0 {
				t.T.Logf("%s: pod list emtpy so daemonset is down", time.Now().String())
				return true, nil
			}
			for _, pod := range podList.Items {
				t.T.Logf("%s: pod %s has status %s and delete timestamp %v\n", time.Now().String(), pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
				if pod.DeletionTimestamp == nil {
					t.T.Logf("%s: pod %s still does not have a deletion timestamp\n", time.Now().String(), pod.Name)
					return false, nil
				}
			}
			t.T.Logf("%s: all daemonset pods are either gone or have deletion timestamp", time.Now().String())
		}
		return true, nil
	})
	return err
}

func RestartDaemonSet(t *TestArgs) {
	t.T.Logf("%s: deleting daemonset pods", time.Now().String())
	err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.T.Logf("%s: unexpected error list driver pods: %s", time.Now().String(), err.Error())
			return false, nil
		}
		for _, pod := range podList.Items {
			err = podClient.Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			if err != nil {
				t.T.Logf("%s: unexpected error deleting pod %s: %s", time.Now().String(), pod.Name, err.Error())
			}
		}
		return true, nil
	})
	if err != nil {
		t.MessageString = fmt.Sprintf("could not delete driver pods in time: %s", err.Error())
		LogAndDebugTestError(t)
	}

	t.DaemonSetUp = false
	err = WaitForDaemonSet(t)
	if err != nil {
		t.MessageString = "csi driver pod deletion not recognized in time"
		LogAndDebugTestError(t)
	}
	t.T.Logf("%s: csi driver pods deletion confirmed, now waiting on pod recreate", time.Now().String())
	// k8s will recreate pods after delete automatically
	t.DaemonSetUp = true
	err = WaitForDaemonSet(t)
	if err != nil {
		t.MessageString = "csi driver restart not recognized in time"
		LogAndDebugTestError(t)
	}
	t.T.Logf("%s: csi driver pods are up with no deletion timestamps", time.Now().String())
}

//TODO presumably this can go away once we have an OLM based deploy that is also integrated with our CI
// so that repo images built from PRs are used when setting up this driver's daemonset
func CreateCSIDriverPlugin(t *TestArgs) {
	_, err1 := kubeClient.CoreV1().Services(client.DefaultNamespace).Get(context.TODO(), "csi-hostpathplugin", metav1.GetOptions{})
	_, err2 := kubeClient.AppsV1().DaemonSets(client.DefaultNamespace).Get(context.TODO(), "csi-hostpathplugin", metav1.GetOptions{})
	if err1 == nil && err2 == nil {
		return
	}
	if err1 != nil {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "csi-hostpathplugin",
				Namespace: client.DefaultNamespace,
				Labels: map[string]string{
					"app": "csi-hostpathplugin",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "csi-hostpathplugin",
				},
				Ports: []corev1.ServicePort{
					{
						Name: "dummy",
						Port: 12345,
					},
				},
			},
		}
		_, err := kubeClient.CoreV1().Services("csi-driver-projected-resource").Create(context.TODO(), service, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			t.T.Fatalf("problem creating csi driver service: %s", time.Now().String())
		}
	}
	if err2 != nil {
		hostPathDirectoryOrCreateType := corev1.HostPathDirectoryOrCreate
		hostPathDirectoryType := corev1.HostPathDirectory
		truePtr := true
		mountPropPtr := corev1.MountPropagationBidirectional
		imageName := "quay.io/openshift/origin-csi-driver-projected-resource:latest"
		// for local testing override
		imageNameOverride, found := os.LookupEnv("IMAGE_NAME")
		if found {
			t.T.Logf("%s: found local override image %s", time.Now().String(), imageNameOverride)
			imageName = imageNameOverride
		}
		if !found {
			// for CI override
			imageNameOverride, found = os.LookupEnv("IMAGE_FORMAT")
			if found {
				t.T.Logf("%s: found CI image %s", time.Now().String(), imageNameOverride)
				imageNameOverride = strings.ReplaceAll(imageNameOverride, "${component}", "csi-driver-projected-resource")
				imageNameOverride = strings.ReplaceAll(imageNameOverride, "stable", "pipeline")
				imageName = imageNameOverride
			}
		}
		daemonSet := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "csi-hostpathplugin",
				Namespace: client.DefaultNamespace,
				Labels: map[string]string{
					"app": "csi-hostpathplugin",
				},
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "csi-hostpathplugin",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "csi-hostpathplugin",
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: "csi-driver-projected-resource-plugin",
						Volumes: []corev1.Volume{
							{
								Name: "socket-dir",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/lib/kubelet/plugins/csi-hostpath",
										Type: &hostPathDirectoryOrCreateType,
									},
								},
							},
							{
								Name: "mountpoint-dir",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/lib/kubelet/pods",
										Type: &hostPathDirectoryOrCreateType,
									},
								},
							},
							{
								Name: "csi-volumes-map",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/lib/csi-volumes-map/",
										Type: &hostPathDirectoryOrCreateType,
									},
								},
							},
							{
								Name: "registration-dir",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/lib/kubelet/plugins_registry",
										Type: &hostPathDirectoryType,
									},
								},
							},
							{
								Name: "plugins-dir",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/lib/kubelet/plugins",
										Type: &hostPathDirectoryType,
									},
								},
							},
							{
								Name: "dev-dir",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dev",
										Type: &hostPathDirectoryType,
									},
								},
							},
							{
								Name: "csi-data-dir",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{
										Medium: corev1.StorageMediumMemory,
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "node-driver-registrar",
								Image:           "quay.io/openshift/origin-csi-node-driver-registrar:latest",
								ImagePullPolicy: corev1.PullAlways,
								Args: []string{
									"--v=5",
									"--csi-address=/csi/csi.sock",
									"--kubelet-registration-path=/var/lib/kubelet/plugins/csi-hostpath/csi.sock",
								},
								Env: []corev1.EnvVar{
									{
										Name: "KUBE_NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "spec.nodeName",
											},
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "socket-dir",
										MountPath: "/csi",
									},
									{
										Name:      "csi-data-dir",
										MountPath: "/csi-data-dir",
									},
									{
										Name:      "registration-dir",
										MountPath: "/registration",
									},
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &truePtr,
								},
							},
							{
								Name:            "hostpath",
								Image:           imageName,
								ImagePullPolicy: corev1.PullAlways,
								Command:         []string{"csi-driver-projected-resource"},
								Args: []string{
									"--drivername=csi-driver-projected-resource.openshift.io",
									"--v=4",
									"--endpoint=$(CSI_ENDPOINT)",
									"--nodeid=$(KUBE_NODE_NAME)",
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "healthz",
										ContainerPort: 9898,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "CSI_ENDPOINT",
										Value: "unix:///csi/csi.sock",
									},
									{
										Name: "KUBE_NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "spec.nodeName",
											},
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "socket-dir",
										MountPath: "/csi",
									},
									{
										Name:      "csi-data-dir",
										MountPath: "/csi-data-dir",
									},
									{
										Name:      "csi-volumes-map",
										MountPath: "/csi-volumes-map",
									},
									{
										Name:      "dev-dir",
										MountPath: "/dev",
									},
									{
										Name:             "mountpoint-dir",
										MountPath:        "/var/lib/kubelet/pods",
										MountPropagation: &mountPropPtr,
									},
									{
										Name:             "plugins-dir",
										MountPath:        "/var/lib/kubelet/plugins",
										MountPropagation: &mountPropPtr,
									},
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &truePtr,
								},
							},
						},
					},
				},
			},
		}
		_, err := kubeClient.AppsV1().DaemonSets("csi-driver-projected-resource").Create(context.TODO(), daemonSet, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			t.T.Fatalf("problem creating csi driver daemonset: %s", err.Error())
		}
	}
	t.T.Logf("%s: csi driver service and daemonset created", time.Now().String())
}
