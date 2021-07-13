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
	"time"

	"github.com/jaypipes/ghw"
	topologyv1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	"sigs.k8s.io/node-feature-discovery/source"
)

const (
	// obtained these values from node e2e tests : https://github.com/kubernetes/kubernetes/blob/82baa26905c94398a0d19e1b1ecf54eb8acb6029/test/e2e_node/util.go#L70
	defaultPodResourcesTimeout = 10 * time.Second
)

type nodeResources struct {
	perNUMAAllocatable map[int]map[v1.ResourceName]int64
	// mapping: resourceName -> resourceID -> nodeID
	resourceID2NUMAID    map[string]map[string]int
	topo                 *ghw.TopologyInfo
	reservedCPUIDPerNUMA map[int][]string
}

type resourceData struct {
	available   int64
	allocatable int64
	capacity    int64
}

func NewResourcesAggregator(podResourceClient podresourcesapi.PodResourcesListerClient) (ResourcesAggregator, error) {
	var err error

	topo, err := ghw.Topology(ghw.WithPathOverrides(ghw.PathOverrides{
		"/sys": string(source.SysfsDir),
	}))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultPodResourcesTimeout)
	defer cancel()

	// Pod Resource API client
	resp, err := podResourceClient.GetAllocatableResources(ctx, &podresourcesapi.AllocatableResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("can't receive response: %v.Get(_) = _, %w", podResourceClient, err)
	}

	return NewResourcesAggregatorFromData(topo, resp), nil
}

// NewResourcesAggregatorFromData is used to aggregate resource information based on the received data from underlying hardware and podresource API
func NewResourcesAggregatorFromData(topo *ghw.TopologyInfo, resp *podresourcesapi.AllocatableResourcesResponse) ResourcesAggregator {
	allDevs := getContainerDevicesFromAllocatableResources(resp, topo)
	return &nodeResources{
		topo:                 topo,
		resourceID2NUMAID:    makeResourceMap(len(topo.Nodes), allDevs),
		perNUMAAllocatable:   makeNodeAllocatable(allDevs),
		reservedCPUIDPerNUMA: makeReservedCPUMap(topo.Nodes, allDevs),
	}
}

// Aggregate provides the mapping (numa zone name) -> Zone from the given PodResources.
func (noderesourceData *nodeResources) Aggregate(podResData []PodResources) topologyv1alpha1.ZoneList {
	perNuma := make(map[int]map[v1.ResourceName]*resourceData)
	for nodeID := range noderesourceData.topo.Nodes {
		nodeRes, ok := noderesourceData.perNUMAAllocatable[nodeID]
		if ok {
			perNuma[nodeID] = make(map[v1.ResourceName]*resourceData)
			for resName, resCap := range nodeRes {
				if resName == "cpu" {
					perNuma[nodeID][resName] = &resourceData{
						allocatable: resCap,
						available:   resCap,
						capacity:    resCap + int64(len(noderesourceData.reservedCPUIDPerNUMA[nodeID])),
					}
				} else {
					perNuma[nodeID][resName] = &resourceData{
						allocatable: resCap,
						available:   resCap,
						capacity:    resCap,
					}
				}
			}
			// NUMA node doesn't have any allocatable resources, but yet it exists in the topology
			// thus all its CPUs are reserved
		} else {
			perNuma[nodeID] = make(map[v1.ResourceName]*resourceData)
			perNuma[nodeID]["cpu"] = &resourceData{
				allocatable: int64(0),
				available:   int64(0),
				capacity:    int64(len(noderesourceData.reservedCPUIDPerNUMA[nodeID])),
			}
		}
	}

	for _, podRes := range podResData {
		for _, contRes := range podRes.Containers {
			for _, res := range contRes.Resources {
				noderesourceData.updateAvailable(perNuma, res)
			}
		}
	}

	zones := make(topologyv1alpha1.ZoneList, 0)
	for nodeID, resList := range perNuma {
		zone := topologyv1alpha1.Zone{
			Name:      makeZoneName(nodeID),
			Type:      "Node",
			Resources: make(topologyv1alpha1.ResourceInfoList, 0),
		}

		costs, err := makeCostsPerNumaNode(noderesourceData.topo.Nodes, nodeID)
		if err != nil {
			klog.Infof("cannot find costs for NUMA node %d: %v", nodeID, err)
		} else {
			zone.Costs = topologyv1alpha1.CostList(costs)
		}

		for name, resData := range resList {
			allocatableQty := *resource.NewQuantity(resData.allocatable, resource.DecimalSI)
			capacityQty := *resource.NewQuantity(resData.capacity, resource.DecimalSI)
			availableQty := *resource.NewQuantity(resData.available, resource.DecimalSI)
			zone.Resources = append(zone.Resources, topologyv1alpha1.ResourceInfo{
				Name:        name.String(),
				Available:   availableQty,
				Allocatable: allocatableQty,
				Capacity:    capacityQty,
			})
		}
		zones = append(zones, zone)
	}
	return zones
}

// getContainerDevicesFromAllocatableResources normalize all compute resources to ContainerDevices.
// This is helpful because cpuIDs are not represented as ContainerDevices, but with a different format;
// Having a consistent representation of all the resources as ContainerDevices makes it simpler for
func getContainerDevicesFromAllocatableResources(availRes *podresourcesapi.AllocatableResourcesResponse, topo *ghw.TopologyInfo) []*podresourcesapi.ContainerDevices {
	var contDevs []*podresourcesapi.ContainerDevices
	contDevs = append(contDevs, availRes.GetDevices()...)

	cpuIDToNodeIDMap := MakeLogicalCoreIDToNodeIDMap(topo)

	cpusPerNuma := make(map[int][]string)
	for _, cpuID := range availRes.GetCpuIds() {
		nodeID, ok := cpuIDToNodeIDMap[int(cpuID)]
		if !ok {
			klog.Infof("cannot find the NUMA node for CPU %d", cpuID)
			continue
		}

		cpuIDList := cpusPerNuma[nodeID]
		cpuIDList = append(cpuIDList, fmt.Sprintf("%d", cpuID))
		cpusPerNuma[nodeID] = cpuIDList
	}

	for nodeID, cpuList := range cpusPerNuma {
		contDevs = append(contDevs, &podresourcesapi.ContainerDevices{
			ResourceName: string(v1.ResourceCPU),
			DeviceIds:    cpuList,
			Topology: &podresourcesapi.TopologyInfo{
				Nodes: []*podresourcesapi.NUMANode{
					{ID: int64(nodeID)},
				},
			},
		})
	}

	return contDevs
}

// updateAvailable computes the actually available resources.
// This function assumes the available resources are initialized to be equal to the allocatable.
func (noderesourceData *nodeResources) updateAvailable(numaData map[int]map[v1.ResourceName]*resourceData, ri ResourceInfo) {
	for _, resID := range ri.Data {
		resName := string(ri.Name)
		resMap, ok := noderesourceData.resourceID2NUMAID[resName]
		if !ok {
			klog.Infof("unknown resource %q", ri.Name)
			continue
		}
		nodeID, ok := resMap[resID]
		if !ok {
			klog.Infof("unknown resource %q: %q", resName, resID)
			continue
		}
		numaData[nodeID][ri.Name].available--
	}
}

// makeZoneName returns the canonical name of a NUMA zone from its ID.
func makeZoneName(nodeID int) string {
	return fmt.Sprintf("node-%d", nodeID)
}

// makeNodeAllocatable computes the node allocatable as mapping (NUMA node ID) -> Resource -> Allocatable (amount, int).
// The computation is done assuming all the resources to represent the allocatable for are represented on a slice
// of ContainerDevices. No special treatment is done for CPU IDs. See getContainerDevicesFromAllocatableResources.
func makeNodeAllocatable(devices []*podresourcesapi.ContainerDevices) map[int]map[v1.ResourceName]int64 {
	perNUMAAllocatable := make(map[int]map[v1.ResourceName]int64)
	// initialize with the capacities
	for _, device := range devices {
		resourceName := device.GetResourceName()
		for _, node := range device.GetTopology().GetNodes() {
			nodeID := int(node.GetID())
			nodeRes, ok := perNUMAAllocatable[nodeID]
			if !ok {
				nodeRes = make(map[v1.ResourceName]int64)
			}
			nodeRes[v1.ResourceName(resourceName)] += int64(len(device.GetDeviceIds()))
			perNUMAAllocatable[nodeID] = nodeRes
		}
	}
	return perNUMAAllocatable
}

func MakeLogicalCoreIDToNodeIDMap(topo *ghw.TopologyInfo) map[int]int {
	core2node := make(map[int]int)
	for _, node := range topo.Nodes {
		for _, core := range node.Cores {
			for _, procID := range core.LogicalProcessors {
				core2node[procID] = node.ID
			}
		}
	}
	return core2node
}

// makeResourceMap creates the mapping (resource name) -> (device ID) -> (NUMA node ID) from the given slice of ContainerDevices.
// this is useful to quickly learn the NUMA ID of a given (resource, device).
func makeResourceMap(numaNodes int, devices []*podresourcesapi.ContainerDevices) map[string]map[string]int {
	resourceMap := make(map[string]map[string]int)

	for _, device := range devices {
		resourceName := device.GetResourceName()
		_, ok := resourceMap[resourceName]
		if !ok {
			resourceMap[resourceName] = make(map[string]int)
		}
		for _, node := range device.GetTopology().GetNodes() {
			nodeID := int(node.GetID())
			for _, deviceID := range device.GetDeviceIds() {
				resourceMap[resourceName][deviceID] = nodeID
			}
		}
	}
	return resourceMap
}

// makeCostsPerNumaNode builds the cost map to reach all the known NUMA zones (mapping (numa zone) -> cost) starting from the given NUMA zone.
func makeCostsPerNumaNode(nodes []*ghw.TopologyNode, nodeIDSrc int) ([]topologyv1alpha1.CostInfo, error) {
	nodeSrc := findNodeByID(nodes, nodeIDSrc)
	if nodeSrc == nil {
		return nil, fmt.Errorf("unknown node: %d", nodeIDSrc)
	}
	nodeCosts := make([]topologyv1alpha1.CostInfo, 0)
	for nodeIDDst, dist := range nodeSrc.Distances {
		// TODO: this assumes there are no holes (= no offline node) in the distance vector
		nodeCosts = append(nodeCosts, topologyv1alpha1.CostInfo{
			Name:  makeZoneName(nodeIDDst),
			Value: int64(dist),
		})
	}
	return nodeCosts, nil
}

func findNodeByID(nodes []*ghw.TopologyNode, nodeID int) *ghw.TopologyNode {
	for _, node := range nodes {
		if node.ID == nodeID {
			return node
		}
	}
	return nil
}

func makeReservedCPUMap(nodes []*ghw.TopologyNode, devices []*podresourcesapi.ContainerDevices) map[int][]string {
	reservedCPUsPerNuma := make(map[int][]string)
	cpus := getCPUs(devices)
	for _, node := range nodes {
		nodeID := node.ID
		for _, core := range node.Cores {
			for _, cpu := range core.LogicalProcessors {
				cpuID := fmt.Sprintf("%d", cpu)
				_, ok := cpus[cpuID]
				if !ok {
					cpuIDList, ok := reservedCPUsPerNuma[nodeID]
					if !ok {
						cpuIDList = make([]string, 0)
					}
					cpuIDList = append(cpuIDList, cpuID)
					reservedCPUsPerNuma[nodeID] = cpuIDList
				}
			}
		}
	}
	return reservedCPUsPerNuma
}

func getCPUs(devices []*podresourcesapi.ContainerDevices) map[string]int {
	cpuMap := make(map[string]int)
	for _, device := range devices {
		if device.GetResourceName() == "cpu" {
			for _, devId := range device.DeviceIds {
				cpuMap[devId] = int(device.Topology.Nodes[0].ID)
			}
		}
	}
	return cpuMap
}
