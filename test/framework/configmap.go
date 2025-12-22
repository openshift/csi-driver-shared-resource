package framework

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

const (
	noRefreshConfgYaml = `
---
refreshResources: false
`
)

func TurnOffRefreshResources(t *TestArgs) {
	cmClient := kubeClient.CoreV1().ConfigMaps(client.DefaultNamespace)
	var cm *corev1.ConfigMap
	ctx := context.TODO()
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Second, true,
		func(ctx context.Context) (done bool, err error) {
			cm, err = cmClient.Get(context.TODO(), client.DriverConfigurationConfigMap, metav1.GetOptions{})
			if err != nil {
				t.T.Logf("%s: error getting driver config configmap: %v", time.Now().String(), err)
				return false, nil
			}
			t.T.Logf("%s: found driver config configmap", time.Now().String())
			return true, nil
		})
	if err != nil {
		// try to create
		//TODO eventually when BUILD-340 is done operator should guarantee this CM exists
		cm = &corev1.ConfigMap{}
		cm.Name = client.DriverConfigurationConfigMap
		cm.Name = client.DefaultNamespace
		cm.Data = map[string]string{}
		cm.Data[client.DriverConfigurationDataKey] = noRefreshConfgYaml

		_, err = cmClient.Create(context.TODO(), cm, metav1.CreateOptions{})
		if err != nil {
			t.MessageString = "unable to create configuration configmap after not locating it"
			LogAndDebugTestError(t)
		}
		return
	}

	// update config
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Second, true,
		func(ctx context.Context) (done bool, err error) {
			cm, err = cmClient.Get(context.TODO(), client.DriverConfigurationConfigMap, metav1.GetOptions{})
			if err != nil {
				t.T.Logf("%s: error getting driver config configmap for update: %v", time.Now().String(), err)
				return false, nil
			}
			if cm.Data == nil {
				cm.Data = map[string]string{}
			}
			cm.Data[client.DriverConfigurationDataKey] = noRefreshConfgYaml
			_, err = cmClient.Update(context.TODO(), cm, metav1.UpdateOptions{})
			if err != nil {
				t.T.Logf("%s: error updating driver config configmap: %v", time.Now().String(), err)
			}
			t.T.Logf("%s: updated driver config configmap", time.Now().String())
			return true, nil
		})
	if err != nil {
		t.MessageString = "unable to change config to turn of refresh"
		LogAndDebugTestError(t)
	}

}
