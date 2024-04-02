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
	corev1 "k8s.io/api/core/v1"

	"k8s.io/utils/ptr"
)

func getWorkerAffinity() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "node-role.kubernetes.io/master",
								Operator: "DoesNotExist",
							},
						},
					},
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "node-role.kubernetes.io/node",
								Operator: "Exists",
							},
						},
					},
				},
			},
		},
	}
}

func getWorkerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: "RuntimeDefault",
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func getWorkerVolumeMounts() *[]corev1.VolumeMount {

	containerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "host-boot",
			MountPath: "/host-boot",
			ReadOnly:  true,
		},
		{
			Name:      "host-os-release",
			MountPath: "/host-etc/os-release",
			ReadOnly:  true,
		},
		{
			Name:      "host-sys",
			MountPath: "/host-sys",
		},
		{
			Name:      "nfd-worker-config",
			MountPath: "/etc/kubernetes/node-feature-discovery",
		},
		{
			Name:      "nfd-hooks",
			MountPath: "/etc/kubernetes/node-feature-discovery/source.d",
		},
		{
			Name:      "nfd-features",
			MountPath: "/etc/kubernetes/node-feature-discovery/features.d",
		},
		{
			Name:      "host-usr-lib",
			MountPath: "/host-usr/lib",
			ReadOnly:  true,
		},
		{
			Name:      "host-lib",
			MountPath: "/host-lib",
			ReadOnly:  true,
		},
		{
			Name:      "host-usr-src",
			MountPath: "/host-usr/src",
			ReadOnly:  true,
		},
		{
			Name:      "host-proc-swaps",
			MountPath: "/host-proc/swaps",
			ReadOnly:  true,
		},
	}

	return &containerVolumeMounts
}

func getWorkerVolumes() []corev1.Volume {
	containerVolume := []corev1.Volume{
		{
			Name: "host-boot",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/boot",
				},
			},
		},
		{
			Name: "host-os-release",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/os-release",
				},
			},
		},
		{
			Name: "host-sys",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys",
				},
			},
		},
		{
			Name: "nfd-hooks",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/kubernetes/node-feature-discovery/source.d",
				},
			},
		},
		{
			Name: "nfd-features",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/kubernetes/node-feature-discovery/features.d",
				},
			},
		},
		{
			Name: "nfd-worker-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "nfd-worker"},
					Items: []corev1.KeyToPath{
						{
							Key:  "nfd-worker-conf",
							Path: "nfd-worker.conf",
						},
					},
				},
			},
		},
		{
			Name: "host-usr-lib",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/host-usr/lib",
				},
			},
		},
		{
			Name: "host-lib",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/host-lib",
				},
			},
		},
		{
			Name: "host-usr-src",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/host-usr/src",
				},
			},
		},
		{
			Name: "host-proc-swaps",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/host-proc/swaps",
				},
			},
		},
	}
	return containerVolume
}

func getWorkerLabelsAForApp(name string) map[string]string {
	return map[string]string{"app": name}
}
