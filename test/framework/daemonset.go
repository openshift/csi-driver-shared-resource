package framework

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/csi-driver-projected-resource/pkg/client"
)

func WaitForDaemonSet(up bool) error {
	dsClient := kubeClient.AppsV1().DaemonSets(client.DefaultNamespace)
	err := wait.PollImmediate(1*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := dsClient.Get(context.TODO(), "csi-hostpathplugin", metav1.GetOptions{})
		if err != nil {
			fmt.Printf("error waiting for driver daemonset to exist: %v\n", err)
			return false, nil
		}
		fmt.Println("found operator deployment")
		return true, nil
	})
	podClient = kubeClient.CoreV1().Pods(client.DefaultNamespace)
	err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("error listing pods: %s\n", err.Error())
			return false, nil
		}

		if up {
			if podList.Items == nil || len(podList.Items) != 3 {
				fmt.Printf("number of pods not yet at 3\n")
				return false, nil
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
					fmt.Printf("pod %s in phase %s with deletion timestamp %v\n", pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
					return false, nil
				}
			}
			fmt.Printf("all 3 daemonset pods are running\n")
		} else {
			if podList.Items == nil || len(podList.Items) == 0 {
				fmt.Printf("pod list emtpy so daemonset is down\n")
				return true, nil
			}
			for _, pod := range podList.Items {
				fmt.Printf("pod %s has status %s and delete timestamp %v\n", pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
				if pod.DeletionTimestamp == nil {
					fmt.Printf("pod %s still does not have a deletion timestamp\n", pod.Name)
					return false, nil
				}
			}
			fmt.Printf("all daemonset pods are either gone or have deletion timestamp\n")
		}
		return true, nil
	})
	return err
}

func RestartDaemonSet(t *testing.T) {
	err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("unexpected error list driver pods: %s", err.Error())
			return false, nil
		}
		for _, pod := range podList.Items {
			err = podClient.Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			if err != nil {
				fmt.Printf("unexpected error deleting pod %s: %s", pod.Name, err.Error())
			}
		}
		return true, nil
	})
	if err != nil {
		LogAndDebugTestError(fmt.Sprintf("could not delete driver pods in time: %s", err.Error()), t)
	}

	err = WaitForDaemonSet(false)
	if err != nil {
		LogAndDebugTestError("csi driver pod deletion not recognized in time", t)
	}
	t.Logf("csi driver pods deletion confirmed, now waiting on pod recreate")
	// k8s will recreate pods after delete automatically
	err = WaitForDaemonSet(true)
	if err != nil {
		LogAndDebugTestError("csi driver restart not recognized in time", t)
	}
}
