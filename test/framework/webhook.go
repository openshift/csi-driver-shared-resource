package framework

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

var webhookSetReplicas = 1

func WaitForWebhook(t *TestArgs) error {
	dsClient := kubeClient.AppsV1().Deployments(client.DefaultNamespace)
	err := wait.PollImmediate(1*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := dsClient.Get(context.TODO(), "shared-resource-csi-driver-webhook", metav1.GetOptions{})
		if err != nil {
			t.T.Logf("%s: error waiting for shared-resource-csi-driver-webhook deployment to exist: %v", time.Now().String(), err)
			return false, nil
		}
		t.T.Logf("%s: found shared-resource-csi-driver-webhook deployment", time.Now().String())
		return true, nil
	})
	podClient = kubeClient.CoreV1().Pods(client.DefaultNamespace)
	err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{LabelSelector: "name=shared-resource-csi-driver-webhook"})
		if err != nil {
			t.T.Logf("%s: error listing shared-resource-csi-driver-webhook pods: %s\n", time.Now().String(), err.Error())
			return false, nil
		}

		if t.WebhookUp {
			if podList.Items == nil || len(podList.Items) < webhookSetReplicas {
				t.T.Logf("%s: number of shared-resource-csi-driver-webhook pods not yet at %d", time.Now().String(), webhookSetReplicas)
				return false, nil
			}
			podCount := 0
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, "shared-resource-csi-driver-webhook") {
					podCount++
				} else {
					continue
				}
				if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
					t.T.Logf("%s: pod %s in phase %s with deletion timestamp %v\n", time.Now().String(), pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
					return false, nil
				}
				if podCount < webhookSetReplicas {
					t.T.Logf("%s: number of shared-resource-csi-driver-webhook pods not yet at %d", time.Now().String(), webhookSetReplicas)
					continue
				}
			}
			t.T.Logf("%s: all shared-resource-csi-driver-webhook pods are running", time.Now().String())
		} else {
			if podList.Items == nil || len(podList.Items) == 0 {
				t.T.Logf("%s: shared-resource-csi-driver-webhook pod list emtpy so shared-resource-csi-driver-webhook is down", time.Now().String())
				return true, nil
			}
			for _, pod := range podList.Items {
				if !strings.HasPrefix(pod.Name, "shared-resource-csi-driver-webhook") {
					continue
				}
				t.T.Logf("%s: pod %s has status %s and delete timestamp %v\n", time.Now().String(), pod.Name, pod.Status.Phase, pod.DeletionTimestamp)
				if pod.DeletionTimestamp == nil {
					t.T.Logf("%s: pod %s still does not have a deletion timestamp\n", time.Now().String(), pod.Name)
					return false, nil
				}
			}
			t.T.Logf("%s: all shared-resource-csi-driver-webhook pods are either gone or have deletion timestamp", time.Now().String())
		}
		return true, nil
	})
	return err
}
