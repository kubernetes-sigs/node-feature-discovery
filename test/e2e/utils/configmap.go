package utils

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

const TopologyUpdaterConfigMapName = "topology-updater-config"

func CreateTopologyUpdaterConfigMap(cs clientset.Interface, ns string, obj interface{})  error{
	eJSONBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	cm := &v1.ConfigMap {
		ObjectMeta: metav1.ObjectMeta {
			Name: TopologyUpdaterConfigMapName,
		},
		Data: map[string]string{
			"exclude-list-config.yaml": string(eJSONBytes[:]),
		},
	}

	cm, err = cs.CoreV1().ConfigMaps(ns).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
