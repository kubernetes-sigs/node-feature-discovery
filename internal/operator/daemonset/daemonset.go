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

package daemonset

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
)

//go:generate mockgen -source=daemonset.go -package=daemonset -destination=mock_daemonset.go DaemonsetAPI

type DaemonsetAPI interface {
	SetTopologyDaemonsetAsDesired(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery, topologyDS *appsv1.DaemonSet) error
}

type daemonset struct {
	scheme *runtime.Scheme
}

func NewDaemonsetAPI(scheme *runtime.Scheme) DaemonsetAPI {
	return &daemonset{
		scheme: scheme,
	}
}

func (d *daemonset) SetTopologyDaemonsetAsDesired(ctx context.Context, nfdInstance *nfdv1.NodeFeatureDiscovery, topologyDS *appsv1.DaemonSet) error {
	topologyDS.ObjectMeta.Labels = map[string]string{"app": "nfd"}

	podLabels := map[string]string{"app": "nfd-topology-updater"}
	topologyDS.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: podLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "nfd-topology-updater",
				DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
				Containers: []corev1.Container{
					{
						Name:            "nfd-topology-updater",
						Image:           nfdInstance.Spec.Operand.ImagePath(),
						ImagePullPolicy: getImagePullPolicy(nfdInstance),
						Command: []string{
							"nfd-topology-updater",
						},
						Args:            getArgs(nfdInstance),
						Env:             getEnvs(),
						SecurityContext: getSecurityContext(),
						VolumeMounts:    getVolumeMounts(),
					},
				},
				Volumes: getVolumes(),
			},
		},
	}
	return controllerutil.SetControllerReference(nfdInstance, topologyDS, d.scheme)
}

func getImagePullPolicy(nfdInstance *nfdv1.NodeFeatureDiscovery) corev1.PullPolicy {
	if nfdInstance.Spec.Operand.ImagePullPolicy != "" {
		return corev1.PullPolicy(nfdInstance.Spec.Operand.ImagePullPolicy)
	}
	return corev1.PullAlways
}

func getArgs(nfdInstance *nfdv1.NodeFeatureDiscovery) []string {
	return []string{
		"-podresources-socket=/host-var/lib/kubelet/pod-resources/kubelet.sock",
		"-sleep-interval=3s",
	}
}

func getEnvs() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func getSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsUser: ptr.To[int64](0),
		SELinuxOptions: &corev1.SELinuxOptions{
			Type: "container_runtime_t",
		},
		ReadOnlyRootFilesystem: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		AllowPrivilegeEscalation: ptr.To(true),
	}
}

func getVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "kubelet-podresources-sock",
			MountPath: "/host-var/lib/kubelet/pod-resources/kubelet.sock",
		},
		{
			Name:      "host-sys",
			MountPath: "/host-sys",
		},
		{
			Name:      "kubelet-state-files",
			MountPath: "/host-var/lib/kubelet",
			ReadOnly:  true,
		},
	}
}

func getVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "kubelet-podresources-sock",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/pod-resources/kubelet.sock",
					Type: ptr.To[corev1.HostPathType](corev1.HostPathSocket),
				},
			},
		},
		{
			Name: "host-sys",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys",
					Type: ptr.To[corev1.HostPathType](corev1.HostPathDirectory),
				},
			},
		},
		{
			Name: "kubelet-state-files",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/kubelet",
					Type: ptr.To[corev1.HostPathType](corev1.HostPathDirectory),
				},
			},
		},
	}
}
