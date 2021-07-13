/*
Copyright 2021 The Kubernetes Authors.

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

package resourcemonitor

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	v1qos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"

	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
)

type PodResourcesScanner struct {
	namespace         string
	podResourceClient podresourcesapi.PodResourcesListerClient
	apihelper         apihelper.APIHelpers
}

func NewPodResourcesScanner(namespace string, podResourceClient podresourcesapi.PodResourcesListerClient, kubeApihelper apihelper.APIHelpers) (ResourcesScanner, error) {
	resourcemonitorInstance := &PodResourcesScanner{
		namespace:         namespace,
		podResourceClient: podResourceClient,
		apihelper:         kubeApihelper,
	}
	if resourcemonitorInstance.namespace != "*" {
		klog.Infof("watching namespace %q", resourcemonitorInstance.namespace)
	} else {
		klog.Infof("watching all namespaces")
	}

	return resourcemonitorInstance, nil
}

// isWatchable tells if the the given namespace should be watched.
func (resMon *PodResourcesScanner) isWatchable(podNamespace string, podName string) (bool, error) {
	cli, err := resMon.apihelper.GetClient()
	if err != nil {
		return false, err
	}
	pod, err := resMon.apihelper.GetPod(cli, podNamespace, podName)
	if err != nil {
		return false, err
	}

	if v1qos.GetPodQOS(pod) != v1.PodQOSGuaranteed {
		return false, nil
	}

	if resMon.namespace == "*" && hasExclusiveCPUs(pod) {
		return true, nil
	}
	// TODO:  add an explicit check for guaranteed pods
	return resMon.namespace == podNamespace && hasExclusiveCPUs(pod), nil
}

// hasExclusiveCPUs returns true if a guranteed pod is allocated exclusive CPUs else returns false.
// In isWatchable() function we check for the pod QoS and proceed if it is guaranteed (i.e. request == limit)
// and hence we only check for request in the function below.
func hasExclusiveCPUs(pod *v1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if _, ok := container.Resources.Requests[v1.ResourceCPU]; !ok {
			continue
		}
		isInitContainerGuaranteed := hasIntegralCPUs(pod, &container)
		if !isInitContainerGuaranteed {
			return false
		}
	}
	for _, container := range pod.Spec.Containers {
		if _, ok := container.Resources.Requests[v1.ResourceCPU]; !ok {
			continue
		}
		isAppContainerGuaranteed := hasIntegralCPUs(pod, &container)
		if !isAppContainerGuaranteed {
			return false
		}
	}
	return true
}

// hasIntegralCPUs returns true if a container in pod is requesting integral CPUs else returns false
func hasIntegralCPUs(pod *v1.Pod, container *v1.Container) bool {
	cpuQuantity := container.Resources.Requests[v1.ResourceCPU]
	return cpuQuantity.Value()*1000 == cpuQuantity.MilliValue()
}

// Scan gathers all the PodResources from the system, using the podresources API client.
func (resMon *PodResourcesScanner) Scan() ([]PodResources, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultPodResourcesTimeout)
	defer cancel()

	// Pod Resource API client
	resp, err := resMon.podResourceClient.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("can't receive response: %v.Get(_) = _, %w", resMon.podResourceClient, err)
	}

	var podResData []PodResources

	for _, podResource := range resp.GetPodResources() {
		isWatchable, err := resMon.isWatchable(podResource.GetNamespace(), podResource.GetName())
		if err != nil {
			return nil, fmt.Errorf("checking if pod in a namespace is watchable, namespace:%v, pod name %v: %v", podResource.GetNamespace(), podResource.GetName(), err)
		}
		if !isWatchable {
			continue
		}

		podRes := PodResources{
			Name:      podResource.GetName(),
			Namespace: podResource.GetNamespace(),
		}

		for _, container := range podResource.GetContainers() {
			contRes := ContainerResources{
				Name: container.Name,
			}

			cpuIDs := container.GetCpuIds()
			if len(cpuIDs) > 0 {
				var resCPUs []string
				for _, cpuID := range container.GetCpuIds() {
					resCPUs = append(resCPUs, fmt.Sprintf("%d", cpuID))
				}
				contRes.Resources = []ResourceInfo{
					{
						Name: v1.ResourceCPU,
						Data: resCPUs,
					},
				}
			}

			for _, device := range container.GetDevices() {
				contRes.Resources = append(contRes.Resources, ResourceInfo{
					Name: v1.ResourceName(device.ResourceName),
					Data: device.DeviceIds,
				})
			}

			if len(contRes.Resources) == 0 {
				continue
			}
			podRes.Containers = append(podRes.Containers, contRes)
		}

		if len(podRes.Containers) == 0 {
			continue
		}

		podResData = append(podResData, podRes)

	}

	return podResData, nil
}
