package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubexec "k8s.io/kubectl/pkg/cmd/exec"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"

	operatorv1 "github.com/openshift/api/operator/v1"
)

const (
	containerName = "my-frontend"
)

func CreateTestPod(t *TestArgs) {
	t.T.Logf("%s: start create test pod %s", time.Now().String(), t.Name)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Name,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "my-csi-volume",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							ReadOnly:         &t.ReadOnly,
							Driver:           string(operatorv1.SharedResourcesCSIDriver),
							VolumeAttributes: map[string]string{"sharedConfigMap": t.Name},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    containerName,
					Image:   "quay.io/quay/busybox",
					Command: []string{"sleep", "1000000"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "my-csi-volume",
							MountPath: "/data",
						},
					},
				},
			},
			ServiceAccountName: "default",
		},
	}
	if t.SecondShare {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "my-csi-volume" + secondShareSuffix,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					ReadOnly:         &t.ReadOnly,
					Driver:           string(operatorv1.SharedResourcesCSIDriver),
					VolumeAttributes: map[string]string{"sharedSecret": t.SecondName},
				},
			},
		})
		mountPath := "/data" + secondShareSuffix
		if t.SecondShareSubDir {
			mountPath = filepath.Join("/data", fmt.Sprintf("data%s", secondShareSuffix))
		}
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "my-csi-volume" + secondShareSuffix,
			MountPath: mountPath,
		})
	}

	podClient := kubeClient.CoreV1().Pods(t.Name)
	_, err := podClient.Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		t.MessageString = fmt.Sprintf("error creating test pod: %s", err.Error())
		LogAndDebugTestError(t)
	}

	t.T.Logf("%s: end create test pod %s", time.Now().String(), t.Name)

	if t.TestPodUp {
		t.T.Logf("%s: start verify test pod %s is up", time.Now().String(), t.Name)
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
			pod, err = podClient.Get(context.TODO(), t.Name, metav1.GetOptions{})
			if err != nil {
				t.T.Logf("%s: error getting pod %s: %s", time.Now().String(), t.Name, err.Error())
			}
			if pod.Status.Phase != corev1.PodRunning {
				t.T.Logf("%s: pod %s only in phase %s\n", time.Now().String(), pod.Name, pod.Status.Phase)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			podJSONBytes, err := json.MarshalIndent(pod, "", "    ")
			if err != nil {
				t.MessageString = fmt.Sprintf("test pod did not reach running state and could not jsonify the pod: %s", err.Error())
				LogAndDebugTestError(t)
			}
			t.MessageString = fmt.Sprintf("test pod did not reach running state: %s", string(podJSONBytes))
			LogAndDebugTestError(t)
		}
		t.T.Logf("%s: done verify test pod %s is up", time.Now().String(), t.Name)
	} else {
		mountFailed(t)
	}
}

func mountFailed(t *TestArgs) {
	t.T.Logf("%s: start check events for mount failure for %s", time.Now().String(), t.Name)
	eventClient := kubeClient.CoreV1().Events(t.Name)
	eventList := &corev1.EventList{}
	var err error
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		eventList, err = eventClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.T.Logf("%s: unable to list events in test namespace %s: %s", time.Now().String(), t.Name, err.Error())
			return false, nil
		}
		for _, event := range eventList.Items {
			t.T.Logf("%s: found event %s in namespace %s", time.Now().String(), event.Reason, t.Name)
			// the constant for FailedMount is in k8s/k8s; refraining for vendoring that in this repo
			if event.Reason == "FailedMount" && event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name == t.Name {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		eventJsonString := ""
		for _, event := range eventList.Items {
			eventJsonBytes, e := json.MarshalIndent(event, "", "    ")
			if e != nil {
				t.T.Logf("%s: could not json marshall %#v", time.Now().String(), event)
			} else {
				eventJsonString = fmt.Sprintf("%s\n%s\n", eventJsonString, string(eventJsonBytes))
			}
		}
		t.MessageString = fmt.Sprintf("did not get expected mount failed event for pod %s, event list: %s", t.Name, eventJsonString)
		LogAndDebugTestError(t)
	}
	t.T.Logf("%s: done check events for mount failure for %s", time.Now().String(), t.Name)
}

func ExecPod(t *TestArgs) {
	pollInterval := 1 * time.Second
	if t.TestDuration != 30*time.Second {
		pollInterval = 1 * time.Minute
	}
	dirs := []string{"/data"}
	switch {
	case t.SecondShare && t.SecondShareSubDir:
		dirs = append(dirs, filepath.Join("/data", fmt.Sprintf("data%s", secondShareSuffix)))
	case t.SecondShare && !t.SecondShareSubDir:
		dirs = append(dirs, "/data"+secondShareSuffix)
	}

	for _, startingPoint := range dirs {
		err := wait.PollImmediate(pollInterval, t.TestDuration, func() (bool, error) {
			req := restClient.Post().Resource("pods").Namespace(t.Name).Name(t.Name).SubResource("exec").
				Param("container", containerName).Param("stdout", "true").Param("stderr", "true").
				Param("command", "ls").Param("command", "-laR").Param("command", startingPoint)

			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			remoteExecutor := kubexec.DefaultRemoteExecutor{}
			err := remoteExecutor.Execute("POST", req.URL(), kubeConfig, nil, out, errOut, false, nil)

			if err != nil {
				t.T.Logf("%s: error with remote exec: %s, errOut: %s", time.Now().String(), err.Error(), errOut)
				return false, nil
			}
			if !t.SearchStringMissing && !strings.Contains(out.String(), t.SearchString) {
				t.T.Logf("%s: directory listing did not have expected output: missing: %v\nout: %s\nerr: %s\n", time.Now().String(), t.SearchStringMissing, out.String(), errOut.String())
				return false, nil
			}
			if t.SearchStringMissing && strings.Contains(out.String(), t.SearchString) {
				t.T.Logf("%s: directory listing did not have expected output: missing: %v\nout: %s\nerr: %s\n", time.Now().String(), t.SearchStringMissing, out.String(), errOut.String())
				return false, nil
			}
			t.T.Logf("%s: final directory listing:\n%s", time.Now().String(), out.String())
			return true, nil
		})
		if err == nil {
			return
		}
	}

	t.MessageString = fmt.Sprintf("directory listing search for %s with missing %v failed", t.SearchString, t.SearchStringMissing)
	LogAndDebugTestError(t)
}

func GetPodContainerRestartCount(t *TestArgs) map[string]int32 {
	podClient := kubeClient.CoreV1().Pods(client.DefaultNamespace)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.T.Fatalf("error list pods %v", err)
	}
	rc := map[string]int32{}
	t.T.Logf("%s: GetPodContainerRestartCount have %d items in list, old restart count %v", time.Now().String(), len(podList.Items), t.CurrentDriverContainerRestartCount)
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "shared-resource-csi-driver-node") {
			for _, cs := range pod.Status.ContainerStatuses {
				if strings.TrimSpace(cs.Name) == "hostpath" {
					t.T.Logf("%s: GetPodContainerRestartCount pod %s hostpath container has restart count %d", time.Now().String(), pod.Name, cs.RestartCount)
					rc[pod.Name] = cs.RestartCount
				}
			}
		}
	}
	return rc
}

func WaitForPodContainerRestart(t *TestArgs) error {
	podClient := kubeClient.CoreV1().Pods(client.DefaultNamespace)
	pollInterval := 1 * time.Second
	if t.TestDuration != 30*time.Second {
		pollInterval = 1 * time.Minute
	}
	t.T.Logf("%s: WaitForPodContainerRestart CurrentDriverContainerRestartCount %v", time.Now().String(), t.CurrentDriverContainerRestartCount)
	err := wait.PollImmediate(pollInterval, t.TestDuration, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.T.Fatalf("error list pods %v", err)
		}
		t.T.Logf("%s: WaitForPodContainerRestart have %d items in list", time.Now().String(), len(podList.Items))
		if len(podList.Items) < 3 {
			return false, nil
		}
		for _, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, "shared-resource-csi-driver-node") {
				if pod.Status.Phase != corev1.PodRunning {
					t.T.Logf("%s: WaitForPodContainerRestart pod %s not in running phase: %s", time.Now().String(), pod.Name, pod.Status.Phase)
				}
				for _, cs := range pod.Status.ContainerStatuses {
					if strings.TrimSpace(cs.Name) == "hostpath" {
						t.T.Logf("%s: WaitForPodContainerRestart pod %s hostpath container has restart count %d", time.Now().String(), pod.Name, cs.RestartCount)
						countBeforeConfigChange, ok := t.CurrentDriverContainerRestartCount[pod.Name]
						if !ok {
							t.T.Logf("%s: WaitForPodContainerRestart pod %s did not have a prior restart count?", time.Now().String(), pod.Name)
							return false, fmt.Errorf("no prior restart count for %s", pod.Name)
						}
						if cs.RestartCount <= countBeforeConfigChange {
							return false, nil
						}
					}
				}
			}

		}
		return true, nil
	})
	return err
}

func SearchCSIPods(t *TestArgs) {
	pollInterval := 1 * time.Second
	if t.TestDuration != 30*time.Second {
		pollInterval = 1 * time.Minute
	}
	err := wait.PollImmediate(pollInterval, t.TestDuration, func() (bool, error) {
		dumpCSIPods(t)

		if !t.SearchStringMissing && !strings.Contains(t.LogContent, t.SearchString) {
			t.T.Logf("%s: csi pod listing did not have expected output: missing: %v\n", time.Now().String(), t.SearchStringMissing)
			return false, nil
		}
		if t.SearchStringMissing && strings.Contains(t.LogContent, t.SearchString) {
			t.T.Logf("%s: directory listing did not have expected output: missing: %v\n", time.Now().String(), t.SearchStringMissing)
			return false, nil
		}
		t.T.Logf("%s: shared resource driver pods are good with search string criteria: missing: %v\n, string: %s\n", time.Now().String(), t.SearchStringMissing, t.SearchString)
		return true, nil
	})
	if err == nil {
		return
	}
	t.MessageString = fmt.Sprintf("%s: csi pod bad missing: %v\n, string: %s\n", time.Now().String(), t.SearchStringMissing, t.SearchString)
	LogAndDebugTestError(t)
}

func dumpCSIPods(t *TestArgs) {
	podClient := kubeClient.CoreV1().Pods(client.DefaultNamespace)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.T.Fatalf("error list pods %v", err)
	}
	t.T.Logf("%s: dumpCSIPods have %d items in list", time.Now().String(), len(podList.Items))
	for _, pod := range podList.Items {
		t.T.Logf("%s: dumpCSIPods looking at pod %s in phase %s", time.Now().String(), pod.Name, pod.Status.Phase)
		if strings.HasPrefix(pod.Name, "shared-resource-csi-driver-node") &&
			pod.Status.Phase == corev1.PodRunning {
			podJsonBytes, _ := json.MarshalIndent(pod, "", "    ")
			t.T.Logf("%s: dumpCSIPods pod json:\n:%s", time.Now().String(), string(podJsonBytes))
			for _, container := range pod.Spec.Containers {
				req := podClient.GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
				readCloser, err := req.Stream(context.TODO())
				if err != nil {
					t.T.Fatalf("error getting pod logs for container %s: %s", container.Name, err.Error())
				}
				b, err := ioutil.ReadAll(readCloser)
				if err != nil {
					t.T.Fatalf("error reading pod stream %s", err.Error())
				}
				podLog := string(b)
				if len(t.SearchString) > 0 {
					t.LogContent = t.LogContent + podLog
				}
				t.T.Logf("%s: pod logs for container %s:  %s", time.Now().String(), container.Name, podLog)
			}
		}
	}
}

func dumpTestPod(t *TestArgs) {
	podClient := kubeClient.CoreV1().Pods(t.Name)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.T.Fatalf("error list pods %v", err)
	}
	t.T.Logf("%s: dumpTestPod have %d items in list", time.Now().String(), len(podList.Items))
	for _, pod := range podList.Items {
		podJsonBytes, _ := json.MarshalIndent(pod, "", "    ")
		t.T.Logf("%s: dumpTestPod pod json:\n:%s", time.Now().String(), string(podJsonBytes))
	}
}

func dumpTestPodEvents(t *TestArgs) {
	eventClient := kubeClient.CoreV1().Events(t.Name)
	eventList, err := eventClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.T.Logf("%s: could not list events for namespace %s", time.Now().String(), t.Name)
		return
	}
	for _, event := range eventList.Items {
		eventJsonBytes, e := json.MarshalIndent(event, "", "    ")
		if e != nil {
			t.T.Logf("%s: could not json marshall %#v", time.Now().String(), event)
		} else {
			eventJsonString := fmt.Sprintf("%s\n", string(eventJsonBytes))
			t.T.Logf("%s: event:\n%s", time.Now().String(), eventJsonString)
		}
	}

}
