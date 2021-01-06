package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubexec "k8s.io/kubectl/pkg/cmd/exec"

	"github.com/openshift/csi-driver-projected-resource/pkg/client"
)

const (
	containerName = "my-frontend"
)

func CreateTestPod(name string, expectSucess bool, t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "my-csi-volume",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:           client.DriverName,
							VolumeAttributes: map[string]string{"share": name},
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

	podClient := kubeClient.CoreV1().Pods(name)
	_, err := podClient.Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		LogAndDebugTestError(fmt.Sprintf("error creating test pod: %s", err.Error()), t)
	}

	if expectSucess {
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
			pod, err = podClient.Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				t.Logf("error getting pod %s: %s", name, err.Error())
			}
			if pod.Status.Phase != corev1.PodRunning {
				t.Logf("pod %s only in phase %s\n", pod.Name, pod.Status.Phase)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			podJSONBytes, err := json.MarshalIndent(pod, "", "    ")
			if err != nil {
				LogAndDebugTestError(fmt.Sprintf("test pod did not reach running state and could not jsonify the pod: %s", err.Error()), t)
			}
			LogAndDebugTestError(fmt.Sprintf("test pod did not reach running state: %s", string(podJSONBytes)), t)
		}
	} else {
		mountFailed(name, t)
	}
}

func mountFailed(name string, t *testing.T) {
	eventClient := kubeClient.CoreV1().Events(name)
	eventList := &corev1.EventList{}
	var err error
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		eventList, err = eventClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Logf("unable to list events in test namespace %s: %s", name, err.Error())
			return false, nil
		}
		for _, event := range eventList.Items {
			t.Logf("found event %s in namespace %s", event.Reason, name)
			// the constant for FailedMount is in k8s/k8s; refraining for vendoring that in this repo
			if event.Reason == "FailedMount" && event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name == name {
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
				t.Logf("could not json marshall %#v", event)
			} else {
				eventJsonString = fmt.Sprintf("%s\n%s\n", eventJsonString, string(eventJsonBytes))
			}
		}
		LogAndDebugTestError(fmt.Sprintf("did not get expected mount failed event for pod %s, event list: %s", name, eventJsonString), t)
	}
}

func ExecPod(name, searchString string, missing bool, totalDuration time.Duration, t *testing.T) {
	pollInterval := 1 * time.Second
	if totalDuration != 30*time.Second {
		pollInterval = 1 * time.Minute
	}
	err := wait.PollImmediate(pollInterval, totalDuration, func() (bool, error) {
		req := restClient.Post().Resource("pods").Namespace(name).Name(name).SubResource("exec").
			Param("container", containerName).Param("stdout", "true").Param("stderr", "true").
			Param("command", "ls").Param("command", "-lR").Param("command", "/data")

		out := &bytes.Buffer{}
		errOut := &bytes.Buffer{}
		remoteExecutor := kubexec.DefaultRemoteExecutor{}
		err := remoteExecutor.Execute("POST", req.URL(), kubeConfig, nil, out, errOut, false, nil)

		if err != nil {
			t.Logf("error with remote exec: %s", err.Error())
			return false, nil
		}
		if !missing && !strings.Contains(out.String(), searchString) {
			t.Logf("directory listing did not have expected output: missing: %v\nout: %s\nerr: %s\n", missing, out.String(), errOut.String())
			return false, nil
		}
		if missing && strings.Contains(out.String(), searchString) {
			t.Logf("directory listing did not have expected output: missing: %v\nout: %s\nerr: %s\n", missing, out.String(), errOut.String())
			return false, nil
		}
		t.Logf("final directory listing:\n%s", out.String())
		return true, nil
	})

	if err != nil {
		LogAndDebugTestError(fmt.Sprintf("directory listing search for %s with missing %v failed", searchString, missing), t)
	}
}

func dumpPod(t *testing.T) {
	podClient := kubeClient.CoreV1().Pods(client.DefaultNamespace)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("error list pods %v", err)
	}
	t.Logf("dumpPods have %d items in list", len(podList.Items))
	for _, pod := range podList.Items {
		t.Logf("dumpPods looking at pod %s in phase %s", pod.Name, pod.Status.Phase)
		if strings.HasPrefix(pod.Name, "csi-hostpath") &&
			pod.Status.Phase == corev1.PodRunning {
			for _, container := range pod.Spec.Containers {
				req := podClient.GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
				readCloser, err := req.Stream(context.TODO())
				if err != nil {
					t.Fatalf("error getting pod logs for container %s: %s", container.Name, err.Error())
				}
				b, err := ioutil.ReadAll(readCloser)
				if err != nil {
					t.Fatalf("error reading pod stream %s", err.Error())
				}
				podLog := string(b)
				t.Logf("pod logs for container %s:  %s", container.Name, podLog)

			}
		}
	}
}
