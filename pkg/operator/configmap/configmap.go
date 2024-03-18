/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configmap

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
)

//go:generate mockgen -source=configmap.go -package=configmap -destination=mock_configmap.go ConfigMapAPI

type ConfigMapAPI interface {
	SetWorkerConfigMapAsDesired(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery, workerCM *corev1.ConfigMap) error
}

type configMap struct {
	scheme *runtime.Scheme
}

func NewConfigMapAPI(scheme *runtime.Scheme) ConfigMapAPI {
	return &configMap{
		scheme: scheme,
	}
}

func (c *configMap) SetWorkerConfigMapAsDesired(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery, cm *corev1.ConfigMap) error {

	cm.Data = map[string]string{"nfd-worker-conf": nfdInstance.Spec.WorkerConfig.ConfigData}

	return controllerutil.SetControllerReference(nfdInstance, cm, c.scheme)
}
