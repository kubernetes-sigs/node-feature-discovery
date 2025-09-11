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
	"encoding/json"
	"log"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jaypipes/ghw"
	. "github.com/smartystreets/goconvey/convey"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"

	topologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"

	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

func TestResourcesAggregator(t *testing.T) {

	fakeTopo := ghw.TopologyInfo{}
	Convey("When recovering test topology from JSON data", t, func() {
		err := json.Unmarshal([]byte(testTopology), &fakeTopo)
		So(err, ShouldBeNil)
	})

	var resAggr ResourcesAggregator

	Convey("When I aggregate the node resources fake data and no pod allocation", t, func() {
		availRes := &v1.AllocatableResourcesResponse{
			Devices: []*v1.ContainerDevices{
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-0"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-1"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-2"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-3"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netBBB-0"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netBBB-1"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/gpu",
					DeviceIds:    []string{"gpuAAA"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
			},
			// CPUId 0 and 1 are missing from the list below to simulate
			// that they are not allocatable CPUs (kube-reserved or system-reserved)
			CpuIds: []int64{
				2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
				12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23,
			},
			Memory: []*v1.ContainerMemory{
				{
					MemoryType: "memory",
					Size:       1024,
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					MemoryType: "memory",
					Size:       1024,
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
				{
					MemoryType: "hugepages-2Mi",
					Size:       1024,
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
			},
		}

		memoryResourcesCapacity := utils.NumaMemoryResources{
			0: map[corev1.ResourceName]int64{
				corev1.ResourceMemory: 2048,
			},
			1: map[corev1.ResourceName]int64{
				corev1.ResourceMemory:                2048,
				corev1.ResourceName("hugepages-2Mi"): 2048,
			},
		}
		resAggr = NewResourcesAggregatorFromData(&fakeTopo, availRes, memoryResourcesCapacity, NewExcludeResourceList(map[string][]string{}, ""))

		Convey("When aggregating resources", func() {
			expected := topologyv1alpha2.ZoneList{
				topologyv1alpha2.Zone{
					Name: "node-0",
					Type: "Node",
					Costs: topologyv1alpha2.CostList{
						topologyv1alpha2.CostInfo{
							Name:  "node-0",
							Value: 10,
						},
						topologyv1alpha2.CostInfo{
							Name:  "node-1",
							Value: 20,
						},
					},
					Resources: topologyv1alpha2.ResourceInfoList{
						topologyv1alpha2.ResourceInfo{
							Name:        "cpu",
							Available:   *resource.NewQuantity(11, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(11, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(12, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "fake.io/net",
							Available:   *resource.NewQuantity(4, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(4, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(4, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "memory",
							Available:   *resource.NewQuantity(1024, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1024, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2048, resource.DecimalSI),
						},
					},
				},
				topologyv1alpha2.Zone{
					Name: "node-1",
					Type: "Node",
					Costs: topologyv1alpha2.CostList{
						topologyv1alpha2.CostInfo{
							Name:  "node-0",
							Value: 20,
						},
						topologyv1alpha2.CostInfo{
							Name:  "node-1",
							Value: 10,
						},
					},
					Resources: topologyv1alpha2.ResourceInfoList{
						topologyv1alpha2.ResourceInfo{
							Name:        "cpu",
							Available:   *resource.NewQuantity(11, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(11, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(12, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "fake.io/gpu",
							Available:   *resource.NewQuantity(1, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(1, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "fake.io/net",
							Available:   *resource.NewQuantity(2, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(2, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "hugepages-2Mi",
							Available:   *resource.NewQuantity(1024, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1024, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2048, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "memory",
							Available:   *resource.NewQuantity(1024, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1024, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2048, resource.DecimalSI),
						},
					},
				},
			}

			res := resAggr.Aggregate(nil) // no pods allocation
			sort.Slice(res, func(i, j int) bool {
				return res[i].Name < res[j].Name
			})
			for _, resource := range res {
				sort.Slice(resource.Costs, func(x, y int) bool {
					return resource.Costs[x].Name < resource.Costs[y].Name
				})
			}
			for _, resource := range res {
				sort.Slice(resource.Resources, func(x, y int) bool {
					return resource.Resources[x].Name < resource.Resources[y].Name
				})
			}

			log.Printf("result=%+v", res)
			log.Printf("expected=%+v", expected)
			log.Printf("diff=%s", cmp.Diff(res, expected))
			So(cmp.Equal(res, expected), ShouldBeTrue)
		})
	})

	Convey("When I aggregate the node resources fake data and some pod allocation", t, func() {
		availRes := &v1.AllocatableResourcesResponse{
			Devices: []*v1.ContainerDevices{
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netBBB"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
				{
					ResourceName: "fake.io/gpu",
					DeviceIds:    []string{"gpuAAA"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
			},
			// CPUId 0 is missing from the list below to simulate
			// that it not allocatable (kube-reserved or system-reserved)
			CpuIds: []int64{
				1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
				12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23,
			},
			Memory: []*v1.ContainerMemory{
				{
					MemoryType: "memory",
					Size:       1024,
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 0,
							},
						},
					},
				},
				{
					MemoryType: "memory",
					Size:       1024,
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
				{
					MemoryType: "hugepages-2Mi",
					Size:       1024,
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							{
								ID: 1,
							},
						},
					},
				},
			},
		}

		memoryResourcesCapacity := utils.NumaMemoryResources{
			0: map[corev1.ResourceName]int64{
				corev1.ResourceMemory: 2048,
			},
			1: map[corev1.ResourceName]int64{
				corev1.ResourceMemory:                2048,
				corev1.ResourceName("hugepages-2Mi"): 2048,
			},
		}

		resAggr = NewResourcesAggregatorFromData(&fakeTopo, availRes, memoryResourcesCapacity, NewExcludeResourceList(map[string][]string{}, ""))

		Convey("When aggregating resources", func() {
			podRes := []PodResources{
				{
					Name:      "test-pod-0",
					Namespace: "default",
					Containers: []ContainerResources{
						{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								{
									Name: "cpu",
									Data: []string{"5", "7"},
								},
								{
									Name: "fake.io/net",
									Data: []string{"netBBB"},
								},
								{
									Name:        "memory",
									Data:        []string{"512"},
									NumaNodeIds: []int{1},
								},
								{
									Name:        "hugepages-2Mi",
									Data:        []string{"512"},
									NumaNodeIds: []int{1},
								},
							},
						},
					},
				},
			}

			expected := topologyv1alpha2.ZoneList{
				topologyv1alpha2.Zone{
					Name: "node-0",
					Type: "Node",
					Costs: topologyv1alpha2.CostList{
						topologyv1alpha2.CostInfo{
							Name:  "node-0",
							Value: 10,
						},
						topologyv1alpha2.CostInfo{
							Name:  "node-1",
							Value: 20,
						},
					},
					Resources: topologyv1alpha2.ResourceInfoList{
						topologyv1alpha2.ResourceInfo{
							Name:        "cpu",
							Available:   *resource.NewQuantity(11, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(11, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(12, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "fake.io/net",
							Available:   *resource.NewQuantity(1, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(1, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "memory",
							Available:   *resource.NewQuantity(1024, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1024, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2048, resource.DecimalSI),
						},
					},
				},
				topologyv1alpha2.Zone{
					Name: "node-1",
					Type: "Node",
					Costs: topologyv1alpha2.CostList{
						topologyv1alpha2.CostInfo{
							Name:  "node-0",
							Value: 20,
						},
						topologyv1alpha2.CostInfo{
							Name:  "node-1",
							Value: 10,
						},
					},
					Resources: topologyv1alpha2.ResourceInfoList{
						topologyv1alpha2.ResourceInfo{
							Name:        "cpu",
							Available:   resource.MustParse("10"),
							Allocatable: *resource.NewQuantity(12, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(12, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "fake.io/gpu",
							Available:   *resource.NewQuantity(1, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(1, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "fake.io/net",
							Available:   *resource.NewQuantity(0, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(1, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "hugepages-2Mi",
							Available:   *resource.NewQuantity(512, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1024, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2048, resource.DecimalSI),
						},
						topologyv1alpha2.ResourceInfo{
							Name:        "memory",
							Available:   *resource.NewQuantity(512, resource.DecimalSI),
							Allocatable: *resource.NewQuantity(1024, resource.DecimalSI),
							Capacity:    *resource.NewQuantity(2048, resource.DecimalSI),
						},
					},
				},
			}

			res := resAggr.Aggregate(podRes)
			sort.Slice(res, func(i, j int) bool {
				return res[i].Name < res[j].Name
			})
			for _, resource := range res {
				sort.Slice(resource.Costs, func(x, y int) bool {
					return resource.Costs[x].Name < resource.Costs[y].Name
				})
			}
			for _, resource := range res {
				sort.Slice(resource.Resources, func(x, y int) bool {
					return resource.Resources[x].Name < resource.Resources[y].Name
				})
			}
			log.Printf("result=%+v", res)
			log.Printf("expected=%+v", expected)
			log.Printf("diff=%s", cmp.Diff(res, expected))
			So(cmp.Equal(res, expected), ShouldBeTrue)
		})
	})

}

// ghwc topology -f json
var testTopology = `{
    "nodes": [
      {
        "id": 0,
        "cores": [
          {
            "id": 0,
            "index": 0,
            "total_threads": 2,
            "logical_processors": [
              0,
              12
            ]
          },
          {
            "id": 10,
            "index": 1,
            "total_threads": 2,
            "logical_processors": [
              10,
              22
            ]
          },
          {
            "id": 1,
            "index": 2,
            "total_threads": 2,
            "logical_processors": [
              14,
              2
            ]
          },
          {
            "id": 2,
            "index": 3,
            "total_threads": 2,
            "logical_processors": [
              16,
              4
            ]
          },
          {
            "id": 8,
            "index": 4,
            "total_threads": 2,
            "logical_processors": [
              18,
              6
            ]
          },
          {
            "id": 9,
            "index": 5,
            "total_threads": 2,
            "logical_processors": [
              20,
              8
            ]
          }
        ],
        "distances": [
          10,
          20
        ]
      },
      {
        "id": 1,
        "cores": [
          {
            "id": 0,
            "index": 0,
            "total_threads": 2,
            "logical_processors": [
              1,
              13
            ]
          },
          {
            "id": 10,
            "index": 1,
            "total_threads": 2,
            "logical_processors": [
              11,
              23
            ]
          },
          {
            "id": 1,
            "index": 2,
            "total_threads": 2,
            "logical_processors": [
              15,
              3
            ]
          },
          {
            "id": 2,
            "index": 3,
            "total_threads": 2,
            "logical_processors": [
              17,
              5
            ]
          },
          {
            "id": 8,
            "index": 4,
            "total_threads": 2,
            "logical_processors": [
              19,
              7
            ]
          },
          {
            "id": 9,
            "index": 5,
            "total_threads": 2,
            "logical_processors": [
              21,
              9
            ]
          }
        ],
        "distances": [
          20,
          10
        ]
      }
    ]
}`
