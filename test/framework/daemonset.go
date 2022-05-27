package framework

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

var daemonSetReplicas = 3

func init() {
	dsReplicas := os.Getenv("DAEMONSET_PODS")
	if dsReplicas == "" {
		return
	}

	i, err := strconv.Atoi(dsReplicas)
	if err != nil {
		return
	}
	daemonSetReplicas = i
}

func WaitForDaemonSet(t *TestArgs) error {
	dsClient := kubeClient.AppsV1().DaemonSets(client.DefaultNamespace)
	err := wait.PollImmediate(1*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := dsClient.Get(context.TODO(), "shared-resource-csi-driver-node", metav1.GetOptions{})
		if err != nil {
			t.T.Logf("%s: error waiting for driver daemonset to exist: %v", time.Now().String(), err)
			return false, nil
		}
		t.T.Logf("%s: found operator deployment", time.Now().String())
		return true, nil
	})
	podClient = kubeClient.CoreV1().Pods(client.DefaultNamespace)
	err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{LabelSelector: "app=shared-resource-csi-driver-node"})

		if err != nil {
			t.T.Logf("%s: error listing shared-resource-csi-driver-node pods: %s\n", time.Now().String(), err.Error())
			return false, nil
		}

		if t.DaemonSetUp {
			if podList.Items == nil || len(podList.Items) < daemonSetReplicas {
				t.T.Logf("%s: number of shared-resource-csi-driver-node pods not yet at %d", time.Now().String(), daemonSetReplicas)
				return false, nil
			}
			podCount := 0
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, "shared-resource-csi-driver-node") {
					podCount++
				} else {
					continue
				}
				if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
					t.T.Logf("%s: shared-resource-csi-driver-node pod %s in phase %s with deletion timestamp %v\n", time.Now().String(), pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
					return false, nil
				}
				if podCount < daemonSetReplicas {
					t.T.Logf("%s: number of shared-resource-csi-driver-node pods not yet at %d", time.Now().String(), daemonSetReplicas)
					continue
				}
			}
			t.T.Logf("%s: all 3 daemonset pods are running", time.Now().String())
		} else {
			if podList.Items == nil || len(podList.Items) == 0 {
				t.T.Logf("%s: shared-resource-csi-driver-node pod list emtpy so daemonset is down", time.Now().String())
				return true, nil
			}
			for _, pod := range podList.Items {
				if !strings.HasPrefix(pod.Name, "shared-resource-csi-driver-node") {
					continue
				}
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
