/*
Copyright 2018-2022 The Kubernetes Authors.

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
	"context"
	"flag"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/podutils"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/pointer"
)

var pullIfNotPresent = flag.Bool("nfd.pull-if-not-present", false, "Pull Images if not present - not always")

const (
	PauseImage = "registry.k8s.io/pause"
)

// GuarenteedSleeperPod makes a Guaranteed QoS class Pod object which long enough forever but requires `cpuLimit` exclusive CPUs.
func GuaranteedSleeperPod(cpuLimit string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sleeper-gu-pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "sleeper-gu-cnt",
					Image: PauseImage,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							// we use 1 core because that's the minimal meaningful quantity
							corev1.ResourceName(corev1.ResourceCPU): resource.MustParse(cpuLimit),
							// any random reasonable amount is fine
							corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("100Mi"),
						},
					},
				},
			},
		},
	}
}

// BestEffortSleeperPod makes a Best Effort QoS class Pod object which sleeps long enough
func BestEffortSleeperPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sleeper-be-pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "sleeper-be-cnt",
					Image: PauseImage,
				},
			},
		},
	}
}

// DeletePodsAsync concurrently deletes all the pods in the given name:pod_object mapping. Returns when the longer operation ends.
func DeletePodsAsync(f *framework.Framework, podMap map[string]*corev1.Pod) {
	var wg sync.WaitGroup
	for _, pod := range podMap {
		wg.Add(1)
		go func(podNS, podName string) {
			defer ginkgo.GinkgoRecover()
			defer wg.Done()

			DeletePodSyncByName(f, podName)
		}(pod.Namespace, pod.Name)
	}
	wg.Wait()
}

// DeletePodSyncByName deletes the pod identified by `podName` in the current namespace
func DeletePodSyncByName(f *framework.Framework, podName string) {
	gp := int64(0)
	delOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &gp,
	}
	f.PodClient().DeleteSync(podName, delOpts, framework.DefaultPodDeletionTimeout)
}

type PodSpecOption func(spec *corev1.PodSpec)

// NFDMasterPod provide NFD master pod definition
func NFDMasterPod(opts ...PodSpecOption) *corev1.Pod {
	yes := true
	no := false
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "nfd-master-",
			Labels:       map[string]string{"name": "nfd-master-e2e"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "node-feature-discovery",
					ImagePullPolicy: pullPolicy(),
					Command:         []string{"nfd-master"},
					Env: []corev1.EnvVar{
						{
							Name: "NODE_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						Privileged:               &no,
						RunAsNonRoot:             &yes,
						ReadOnlyRootFilesystem:   &yes,
						AllowPrivilegeEscalation: &no,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			ServiceAccountName: "nfd-master-e2e",
			RestartPolicy:      corev1.RestartPolicyNever,
		},
	}

	for _, o := range opts {
		o(&p.Spec)
	}
	return p
}

// NFDWorkerPod provides NFD worker pod definition
func NFDWorkerPod(opts ...PodSpecOption) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-worker-" + string(uuid.NewUUID()),
		},
		Spec: *nfdWorkerPodSpec(opts...),
	}

	p.Spec.RestartPolicy = corev1.RestartPolicyNever

	return p
}

// NFDWorkerDaemonSet provides the NFD daemon set worker definition
func NFDWorkerDaemonSet(opts ...PodSpecOption) *appsv1.DaemonSet {
	return newDaemonSet("nfd-worker", nfdWorkerPodSpec(opts...))
}

// NFDTopologyUpdaterDaemonSet provides the NFD daemon set topology updater
func NFDTopologyUpdaterDaemonSet(kc KubeletConfig, opts ...PodSpecOption) *appsv1.DaemonSet {
	return newDaemonSet("nfd-topology-updater", nfdTopologyUpdaterPodSpec(kc, opts...))
}

// SpecWithContainerImage returns a PodSpecOption that sets the image used by the first container.
func SpecWithContainerImage(image string) PodSpecOption {
	return func(spec *corev1.PodSpec) {
		// NOTE: we might want to make the container number a parameter
		cnt := &spec.Containers[0]
		cnt.Image = image
	}
}

// SpecWithContainerExtraArgs returns a PodSpecOption that adds extra args to the first container.
func SpecWithContainerExtraArgs(args ...string) PodSpecOption {
	return func(spec *corev1.PodSpec) {
		// NOTE: we might want to make the container number a parameter
		cnt := &spec.Containers[0]
		cnt.Args = append(cnt.Args, args...)
	}
}

// SpecWithMasterNodeSelector returns a PodSpecOption that modifies the pod to
// be run on a control plane node of the cluster.
func SpecWithMasterNodeSelector(args ...string) PodSpecOption {
	return func(spec *corev1.PodSpec) {
		spec.NodeSelector["node-role.kubernetes.io/control-plane"] = ""
		spec.Tolerations = append(spec.Tolerations,
			corev1.Toleration{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: corev1.TolerationOpEqual,
				Value:    "",
				Effect:   corev1.TaintEffectNoSchedule,
			})
	}
}

// SpecWithConfigMap returns a PodSpecOption that mounts a configmap to the first container.
func SpecWithConfigMap(name, mountPath string) PodSpecOption {
	return func(spec *corev1.PodSpec) {
		spec.Volumes = append(spec.Volumes,
			corev1.Volume{
				Name: name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: name,
						},
					},
				},
			})
		cnt := &spec.Containers[0]
		cnt.VolumeMounts = append(cnt.VolumeMounts,
			corev1.VolumeMount{
				Name:      name,
				ReadOnly:  true,
				MountPath: mountPath,
			})
	}
}

// newDaemonSet provide the new daemon set
func newDaemonSet(name string, podSpec *corev1.PodSpec) *appsv1.DaemonSet {
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

func nfdWorkerPodSpec(opts ...PodSpecOption) *corev1.PodSpec {
	yes := true
	no := false
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "node-feature-discovery",
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-worker"},
				Args:            []string{"-server=nfd-master-e2e:8080"},
				Env: []corev1.EnvVar{
					{
						Name: "NODE_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					Privileged:               &no,
					RunAsNonRoot:             &yes,
					ReadOnlyRootFilesystem:   &yes,
					AllowPrivilegeEscalation: &no,
				},
				VolumeMounts: []corev1.VolumeMount{
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
					{
						Name:      "host-usr-lib",
						MountPath: "/host-usr/lib",
						ReadOnly:  true,
					},
					{
						Name:      "host-usr-src",
						MountPath: "/host-usr/src",
						ReadOnly:  true,
					},
				},
			},
		},
		ServiceAccountName: "nfd-master-e2e",
		DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
		Volumes: []corev1.Volume{
			{
				Name: "host-boot",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/boot",
						Type: newHostPathType(corev1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-os-release",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/os-release",
						Type: newHostPathType(corev1.HostPathFile),
					},
				},
			},
			{
				Name: "host-sys",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/sys",
						Type: newHostPathType(corev1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-usr-lib",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/usr/lib",
						Type: newHostPathType(corev1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-usr-src",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/usr/src",
						Type: newHostPathType(corev1.HostPathDirectory),
					},
				},
			},
		},
	}

	for _, o := range opts {
		o(p)
	}
	return p
}

func nfdTopologyUpdaterPodSpec(kc KubeletConfig, opts ...PodSpecOption) *corev1.PodSpec {
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "node-topology-updater",
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-topology-updater"},
				Args: []string{
					"--kubelet-config-uri=file:///podresources/config.yaml",
					"--podresources-socket=unix:///podresources/kubelet.sock",
					"--sleep-interval=3s",
					"--watch-namespace=rte",
					"--server=nfd-master-e2e:8080"},
				Env: []corev1.EnvVar{
					{
						Name: "NODE_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					RunAsUser:                pointer.Int64Ptr(0),
					ReadOnlyRootFilesystem:   pointer.BoolPtr(true),
					AllowPrivilegeEscalation: pointer.BoolPtr(false),
				},
				VolumeMounts: []corev1.VolumeMount{
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
		ServiceAccountName: "nfd-topology-updater-e2e",
		DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
		Volumes: []corev1.Volume{
			{
				Name: "kubelet-podresources-conf",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: kc.ConfigPath,
						Type: newHostPathType(corev1.HostPathFile),
					},
				},
			},
			{
				Name: "kubelet-podresources-sock",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: kc.PodResourcesSocketPath,
						Type: newHostPathType(corev1.HostPathSocket),
					},
				},
			},
			{
				Name: "host-sys",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/sys",
						Type: newHostPathType(corev1.HostPathDirectory),
					},
				},
			},
		},
	}

	for _, o := range opts {
		o(p)
	}
	return p
}

func newHostPathType(typ corev1.HostPathType) *corev1.HostPathType {
	hostPathType := new(corev1.HostPathType)
	*hostPathType = typ
	return hostPathType
}

// WaitForPodsReady waits for the pods to become ready.
// NOTE: copied from k8s v1.22 after which is was removed from there.
// Convenient for checking that all pods of a daemonset are ready.
func WaitForPodsReady(c clientset.Interface, ns, name string, minReadySeconds int) error {
	const poll = 2 * time.Second
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": name}))
	options := metav1.ListOptions{LabelSelector: label.String()}
	return wait.Poll(poll, 5*time.Minute, func() (bool, error) {
		pods, err := c.CoreV1().Pods(ns).List(context.TODO(), options)
		if err != nil {
			return false, nil
		}
		for _, pod := range pods.Items {
			if !podutils.IsPodAvailable(&pod, int32(minReadySeconds), metav1.Now()) {
				return false, nil
			}
		}
		return true, nil
	})
}

func pullPolicy() corev1.PullPolicy {
	if *pullIfNotPresent {
		return corev1.PullIfNotPresent
	}
	return corev1.PullAlways
}
