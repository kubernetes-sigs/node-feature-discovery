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
	"k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	"github.com/k8stopologyawareschedwg/podfingerprint"
)

type PodResourcesScanner struct {
	namespace         string
	podResourceClient podresourcesapi.PodResourcesListerClient
	k8sClient         client.Interface
	podFingerprint    bool
}

// NewPodResourcesScanner creates a new ResourcesScanner instance
func NewPodResourcesScanner(namespace string, podResourceClient podresourcesapi.PodResourcesListerClient, k8sClient client.Interface, podFingerprint bool) (ResourcesScanner, error) {
	resourcemonitorInstance := &PodResourcesScanner{
		namespace:         namespace,
		podResourceClient: podResourceClient,
		k8sClient:         k8sClient,
		podFingerprint:    podFingerprint,
	}
	if resourcemonitorInstance.namespace != "*" {
		klog.InfoS("watching one namespace", "namespace", resourcemonitorInstance.namespace)
	} else {
		klog.InfoS("watching all namespaces")
	}

	return resourcemonitorInstance, nil
}

// isWatchable tells if the the given namespace should be watched.
// In Scan(), if watchable is false, this pods scan will skip
// so we can return directly if pod's namespace is not watchable
func (resMon *PodResourcesScanner) isWatchable(podResource *podresourcesapi.PodResources) (bool, bool, error) {
	if resMon.namespace != "*" && resMon.namespace != podResource.Namespace {
		return false, false, nil
	}

	pod, err := resMon.k8sClient.CoreV1().Pods(podResource.Namespace).Get(context.TODO(), podResource.Name, metav1.GetOptions{})
	if err != nil {
		return false, false, err
	}

	podHasExclusiveCPUs := hasExclusiveCPUs(pod)
	isPodGuaranteed := qos.GetPodQOS(pod) == corev1.PodQOSGuaranteed

	return isPodGuaranteed || hasDevice(podResource), podHasExclusiveCPUs, nil
}

// hasExclusiveCPUs returns true if a guaranteed pod is allocated exclusive CPUs else returns false.
// In isWatchable() function we check for the pod QoS and proceed if it is guaranteed (i.e. request == limit)
// and hence we only check for request in the function below.
func hasExclusiveCPUs(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if hasIntegralCPUs(&container) {
			return true
		}
	}
	for _, container := range pod.Spec.Containers {
		if hasIntegralCPUs(&container) {
			return true
		}
	}

	//No integralCPUs requested in all the containers of the pod
	return false
}

// hasIntegralCPUs returns true if a container in pod is requesting integral CPUs else returns false
func hasIntegralCPUs(container *corev1.Container) bool {
	cpuQuantity, ok := container.Resources.Requests[corev1.ResourceCPU]
	if !ok {
		return false
	}
	return cpuQuantity.Value()*1000 == cpuQuantity.MilliValue()
}

// Scan gathers all the PodResources from the system, using the podresources API client.
func (resMon *PodResourcesScanner) Scan() (ScanResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultPodResourcesTimeout)
	defer cancel()

	// Pod Resource API client
	resp, err := resMon.podResourceClient.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return ScanResponse{}, fmt.Errorf("can't receive response: %v.Get(_) = _, %w", resMon.podResourceClient, err)
	}

	respPodResources := resp.GetPodResources()
	retVal := ScanResponse{
		Attributes: v1alpha2.AttributeList{},
	}

	if resMon.podFingerprint && len(respPodResources) > 0 {
		var status podfingerprint.Status
		podFingerprintSign, err := computePodFingerprint(respPodResources, &status)
		if err != nil {
			klog.ErrorS(err, "failed to calculate fingerprint")
		} else {
			klog.InfoS("podFingerprint calculated", "status", status.Repr())

			retVal.Attributes = append(retVal.Attributes, v1alpha2.AttributeInfo{
				Name:  podfingerprint.Attribute,
				Value: podFingerprintSign,
			})
		}
	}
	var podResData []PodResources

	for _, podResource := range respPodResources {
		klog.InfoS("scanning pod", "podName", podResource.GetName())
		isWatchable, isExclusiveCPUs, err := resMon.isWatchable(podResource)
		if err != nil {
			return ScanResponse{}, fmt.Errorf("checking if pod in a namespace is watchable, namespace:%v, pod name %v: %w", podResource.GetNamespace(), podResource.GetName(), err)
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
			if len(cpuIDs) > 0 && isExclusiveCPUs {
				var resCPUs []string
				for _, cpuID := range container.GetCpuIds() {
					resCPUs = append(resCPUs, strconv.FormatInt(cpuID, 10))
				}
				contRes.Resources = []ResourceInfo{
					{
						Name: corev1.ResourceCPU,
						Data: resCPUs,
					},
				}
			}

			for _, device := range container.GetDevices() {
				numaNodesIDs := getNumaNodeIds(device.GetTopology())
				contRes.Resources = append(contRes.Resources, ResourceInfo{
					Name:        corev1.ResourceName(device.ResourceName),
					Data:        device.DeviceIds,
					NumaNodeIds: numaNodesIDs,
				})
			}

			for _, block := range container.GetMemory() {
				if block.GetSize_() == 0 {
					continue
				}

				topology := getNumaNodeIds(block.GetTopology())
				contRes.Resources = append(contRes.Resources, ResourceInfo{
					Name:        corev1.ResourceName(block.MemoryType),
					Data:        []string{fmt.Sprintf("%d", block.GetSize_())},
					NumaNodeIds: topology,
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

	retVal.PodResources = podResData

	return retVal, nil
}

func hasDevice(podResource *podresourcesapi.PodResources) bool {
	for _, container := range podResource.GetContainers() {
		if len(container.GetDevices()) > 0 {
			return true
		}
	}
	klog.InfoS("pod doesn't have devices", "podName", podResource.GetName())
	return false
}

func getNumaNodeIds(topologyInfo *podresourcesapi.TopologyInfo) []int {
	if topologyInfo == nil {
		return nil
	}

	var topology []int
	for _, node := range topologyInfo.Nodes {
		if node != nil {
			topology = append(topology, int(node.ID))
		}
	}

	return topology
}

func computePodFingerprint(podResources []*podresourcesapi.PodResources, status *podfingerprint.Status) (string, error) {
	fingerprint := podfingerprint.NewTracingFingerprint(len(podResources), status)
	for _, podResource := range podResources {
		err := fingerprint.Add(podResource.Namespace, podResource.Name)
		if err != nil {
			return "", err
		}
	}
	return fingerprint.Sign(), nil
}
