/*
Copyright 2020 The Kubernetes Authors.

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

package utils

import (
	"flag"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
)

var pullIfNotPresent = flag.Bool("nfd.pull-if-not-present", false, "Pull Images if not present - not always")

// NFDMasterPod provide NFD master pod definition
func NFDMasterPod(image string, onMasterNode bool) *v1.Pod {
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "nfd-master-",
			Labels:       map[string]string{"name": "nfd-master-e2e"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "node-feature-discovery",
					Image:           image,
					ImagePullPolicy: pullPolicy(),
					Command:         []string{"nfd-master"},
					Env: []v1.EnvVar{
						{
							Name: "NODE_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
					},
				},
			},
			ServiceAccountName: "nfd-master-e2e",
			RestartPolicy:      v1.RestartPolicyNever,
		},
	}
	if onMasterNode {
		p.Spec.NodeSelector = map[string]string{"node-role.kubernetes.io/master": ""}
		p.Spec.Tolerations = []v1.Toleration{
			{
				Key:      "node-role.kubernetes.io/master",
				Operator: v1.TolerationOpEqual,
				Value:    "",
				Effect:   v1.TaintEffectNoSchedule,
			},
		}
	}
	return p
}

// NFDWorkerPod provides NFD worker pod definition
func NFDWorkerPod(image string, extraArgs []string) *v1.Pod {
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-worker-" + string(uuid.NewUUID()),
		},
		Spec: *nfdWorkerPodSpec(image, extraArgs),
	}

	p.Spec.RestartPolicy = v1.RestartPolicyNever

	return p
}

// NFDWorkerDaemonSet provides the NFD daemon set worker definition
func NFDWorkerDaemonSet(image string, extraArgs []string) *appsv1.DaemonSet {
	podSpec := nfdWorkerPodSpec(image, extraArgs)
	return newDaemonSet("nfd-worker", podSpec)
}

// NFDTopologyUpdaterDaemonSet provides the NFD daemon set topology updater
func NFDTopologyUpdaterDaemonSet(image string, extraArgs []string) *appsv1.DaemonSet {
	podSpec := nfdTopologyUpdaterPodSpec(image, extraArgs)
	return newDaemonSet("nfd-topology-updater", podSpec)
}

// newDaemonSet provide the new daemon set
func newDaemonSet(name string, podSpec *v1.PodSpec) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-" + string(uuid.NewUUID()),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": name},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": name},
				},
				Spec: *podSpec,
			},
			MinReadySeconds: 5,
		},
	}
}

func nfdWorkerPodSpec(image string, extraArgs []string) *v1.PodSpec {
	return &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:            "node-feature-discovery",
				Image:           image,
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-worker"},
				Args:            append([]string{"--server=nfd-master-e2e:8080"}, extraArgs...),
				Env: []v1.EnvVar{
					{
						Name: "NODE_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
				},
				VolumeMounts: []v1.VolumeMount{
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
						ReadOnly:  true,
					},
				},
			},
		},
		ServiceAccountName: "nfd-master-e2e",
		DNSPolicy:          v1.DNSClusterFirstWithHostNet,
		Volumes: []v1.Volume{
			{
				Name: "host-boot",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/boot",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-os-release",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/etc/os-release",
						Type: newHostPathType(v1.HostPathFile),
					},
				},
			},
			{
				Name: "host-sys",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/sys",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
		},
	}
}

func nfdTopologyUpdaterPodSpec(image string, extraArgs []string) *v1.PodSpec {
	return &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:            "node-topology-updater",
				Image:           image,
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-topology-updater"},
				Args: append([]string{
					"--kubelet-config-file=/podresources/config.yaml",
					"--podresources-socket=/podresources/kubelet.sock",
					"--sleep-interval=3s",
					"--watch-namespace=rte",
					"--server=nfd-master-e2e:8080",
				}, extraArgs...),
				Env: []v1.EnvVar{
					{
						Name: "NODE_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
				},
				SecurityContext: &v1.SecurityContext{
					Capabilities: &v1.Capabilities{
						Drop: []v1.Capability{"ALL"},
					},
					RunAsUser:                pointer.Int64Ptr(0),
					ReadOnlyRootFilesystem:   pointer.BoolPtr(true),
					AllowPrivilegeEscalation: pointer.BoolPtr(false),
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "kubelet-podresources-conf",
						MountPath: "/podresources/config.yaml",
					},
					{
						Name:      "kubelet-podresources-sock",
						MountPath: "/podresources/kubelet.sock",
					},
					{
						Name:      "host-sys",
						MountPath: "/host-sys",
					},
				},
			},
		},
		ServiceAccountName: "nfd-master-e2e",
		DNSPolicy:          v1.DNSClusterFirstWithHostNet,
		Volumes: []v1.Volume{
			{
				Name: "kubelet-podresources-conf",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/var/lib/kubelet/config.yaml",
						Type: newHostPathType(v1.HostPathFile),
					},
				},
			},
			{
				Name: "kubelet-podresources-sock",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/var/lib/kubelet/pod-resources/kubelet.sock",
						Type: newHostPathType(v1.HostPathSocket),
					},
				},
			},
			{
				Name: "host-sys",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/sys",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
		},
	}
}

func newHostPathType(typ v1.HostPathType) *v1.HostPathType {
	hostPathType := new(v1.HostPathType)
	*hostPathType = typ
	return hostPathType
}

func pullPolicy() v1.PullPolicy {
	if *pullIfNotPresent {
		return v1.PullIfNotPresent
	}
	return v1.PullAlways
}
