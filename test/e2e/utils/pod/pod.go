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

package pod

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/podutils"
	"k8s.io/utils/ptr"

	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"sigs.k8s.io/node-feature-discovery/test/e2e/utils"
)

var pullIfNotPresent = flag.Bool("nfd.pull-if-not-present", false, "Pull Images if not present - not always")

const (
	PauseImage = "registry.k8s.io/pause"
)

// GuaranteedSleeper  makes a Guaranteed QoS class Pod object which long enough forever but requires `cpuLimit` exclusive CPUs.
func GuaranteedSleeper(opts ...func(pod *corev1.Pod)) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sleeper-gu-pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:      "sleeper-gu-cnt",
					Image:     PauseImage,
					Resources: corev1.ResourceRequirements{},
				},
			},
		},
		Status: corev1.PodStatus{
			QOSClass: corev1.PodQOSGuaranteed,
		},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func WithLimits(list corev1.ResourceList) func(p *corev1.Pod) {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Resources.Limits = list
	}
}

// BestEffortSleeper makes a Best Effort QoS class Pod object which sleeps long enough
func BestEffortSleeper() *corev1.Pod {
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

// DeleteAsync concurrently deletes all the pods in the given name:pod_object mapping. Returns when the longer operation ends.
func DeleteAsync(ctx context.Context, f *framework.Framework, podMap map[string]*corev1.Pod) {
	var wg sync.WaitGroup
	for _, pod := range podMap {
		wg.Add(1)
		go func(podNS, podName string) {
			defer ginkgo.GinkgoRecover()
			defer wg.Done()

			DeleteSyncByName(ctx, f, podName)
		}(pod.Namespace, pod.Name)
	}
	wg.Wait()
}

// DeleteSyncByName deletes the pod identified by `podName` in the current namespace
func DeleteSyncByName(ctx context.Context, f *framework.Framework, podName string) {
	gp := int64(0)
	delOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &gp,
	}
	e2epod.NewPodClient(f).DeleteSync(ctx, podName, delOpts, e2epod.DefaultPodDeletionTimeout)
}

type SpecOption func(spec *corev1.PodSpec)

// NFDMaster provide NFD master pod definition
func NFDMaster(opts ...SpecOption) *corev1.Pod {
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
						Privileged:               ptr.To[bool](false),
						RunAsNonRoot:             ptr.To[bool](true),
						ReadOnlyRootFilesystem:   ptr.To[bool](true),
						AllowPrivilegeEscalation: ptr.To[bool](false),
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

// NFDWorker provides NFD worker pod definition
func NFDWorker(opts ...SpecOption) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-worker-" + string(uuid.NewUUID()),
		},
		Spec: *nfdWorkerSpec(opts...),
	}
	return p
}

// SpecWithRestartPolicy returns a SpecOption that sets the pod restart policy
func SpecWithRestartPolicy(restartpolicy corev1.RestartPolicy) SpecOption {
	return func(spec *corev1.PodSpec) {
		spec.RestartPolicy = restartpolicy
	}
}

// SpecWithContainerImage returns a SpecOption that sets the image used by the first container.
func SpecWithContainerImage(image string) SpecOption {
	return func(spec *corev1.PodSpec) {
		// NOTE: we might want to make the container number a parameter
		cnt := &spec.Containers[0]
		cnt.Image = image
	}
}

// SpecWithContainerExtraArgs returns a SpecOption that adds extra args to the first container.
func SpecWithContainerExtraArgs(args ...string) SpecOption {
	return func(spec *corev1.PodSpec) {
		// NOTE: we might want to make the container number a parameter
		cnt := &spec.Containers[0]
		cnt.Args = append(cnt.Args, args...)
	}
}

// SpecWithMasterNodeSelector returns a SpecOption that modifies the pod to
// be run on a control plane node of the cluster.
func SpecWithMasterNodeSelector(args ...string) SpecOption {
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

// SpecWithTolerations returns a SpecOption that modifies the pod to
// be run on a node with NodeFeatureRule taints.
func SpecWithTolerations(tolerations []corev1.Toleration) SpecOption {
	return func(spec *corev1.PodSpec) {
		spec.Tolerations = append(spec.Tolerations, tolerations...)
	}
}

// SpecWithConfigMap returns a SpecOption that mounts a configmap to the first container.
func SpecWithConfigMap(name, mountPath string) SpecOption {
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

func nfdWorkerSpec(opts ...SpecOption) *corev1.PodSpec {
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "node-feature-discovery",
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-worker"},
				Args:            []string{},
				Env: []corev1.EnvVar{
					{
						Name: "NODE_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
					{
						Name: "POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
					{
						Name: "POD_UID",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.uid",
							},
						},
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					Privileged:               ptr.To[bool](false),
					RunAsNonRoot:             ptr.To[bool](true),
					ReadOnlyRootFilesystem:   ptr.To[bool](true),
					AllowPrivilegeEscalation: ptr.To[bool](false),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
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
				},
			},
		},
		ServiceAccountName: "nfd-worker-e2e",
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
		},
	}

	for _, o := range opts {
		o(p)
	}
	return p
}

func NFDGCSpec(opts ...SpecOption) *corev1.PodSpec {
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "node-feature-discovery",
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-gc"},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					Privileged:               ptr.To[bool](false),
					RunAsNonRoot:             ptr.To[bool](true),
					ReadOnlyRootFilesystem:   ptr.To[bool](true),
					AllowPrivilegeEscalation: ptr.To[bool](false),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
		},
		ServiceAccountName: "nfd-gc-e2e",
	}

	for _, o := range opts {
		o(p)
	}
	return p
}

func NFDTopologyUpdaterSpec(kc utils.KubeletConfig, opts ...SpecOption) *corev1.PodSpec {
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "node-topology-updater",
				ImagePullPolicy: pullPolicy(),
				Command:         []string{"nfd-topology-updater"},
				Args: []string{
					"-kubelet-config-uri=file:///podresources/config.yaml",
					"-podresources-socket=unix:///host-var/lib/kubelet/pod-resources/kubelet.sock",
					"-watch-namespace=rte"},
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
					RunAsUser:                ptr.To[int64](0),
					ReadOnlyRootFilesystem:   ptr.To[bool](true),
					AllowPrivilegeEscalation: ptr.To[bool](false),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "kubelet-state-files",
						MountPath: "/host-var/lib/kubelet",
					},
					{
						Name:      "kubelet-podresources-conf",
						MountPath: "/podresources/config.yaml",
					},
					{
						Name:      "kubelet-podresources-sock",
						MountPath: "/host-var/lib/kubelet/pod-resources/kubelet.sock",
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
				Name: "kubelet-state-files",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/lib/kubelet",
						Type: newHostPathType(corev1.HostPathDirectory),
					},
				},
			},
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

// WaitForReady waits for the pods to become ready.
// NOTE: copied from k8s v1.22 after which is was removed from there.
// Convenient for checking that all pods of a daemonset are ready.
func WaitForReady(ctx context.Context, c clientset.Interface, ns, name string, minReadySeconds int) error {
	const poll = 2 * time.Second
	label := labels.SelectorFromSet(labels.Set(map[string]string{"name": name}))
	options := metav1.ListOptions{LabelSelector: label.String()}
	return wait.PollUntilContextTimeout(ctx, poll, 5*time.Minute, false, func(ctx context.Context) (bool, error) {
		pods, err := c.CoreV1().Pods(ns).List(ctx, options)
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
