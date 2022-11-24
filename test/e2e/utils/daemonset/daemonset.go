/*
Copyright 2022 The Kubernetes Authors.

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

package daemonset

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"sigs.k8s.io/node-feature-discovery/test/e2e/utils"
	"sigs.k8s.io/node-feature-discovery/test/e2e/utils/pod"
)

// NFDWorker provides the NFD daemon set worker definition
func NFDWorker(opts ...pod.SpecOption) *appsv1.DaemonSet {
	return new("nfd-worker", &pod.NFDWorker(opts...).Spec)
}

// NFDTopologyUpdater provides the NFD daemon set topology updater
func NFDTopologyUpdater(kc utils.KubeletConfig, opts ...pod.SpecOption) *appsv1.DaemonSet {
	return new("nfd-topology-updater", pod.NFDTopologyUpdaterSpec(kc, opts...))
}

// new provide the new daemon set
func new(name string, podSpec *corev1.PodSpec) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-" + string(uuid.NewUUID()),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": name},
				},
				Spec: *podSpec,
			},
			MinReadySeconds: 5,
		},
	}
}
