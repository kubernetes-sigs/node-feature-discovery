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

	"github.com/jaypipes/ghw"

	cmp "github.com/google/go-cmp/cmp"
	. "github.com/smartystreets/goconvey/convey"
	"k8s.io/apimachinery/pkg/api/resource"

	topologyv1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"
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
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-0"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 0,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-1"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 0,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-2"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 0,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA-3"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 0,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netBBB-0"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 1,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netBBB-1"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 1,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/gpu",
					DeviceIds:    []string{"gpuAAA"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
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
		}

		resAggr = NewResourcesAggregatorFromData(&fakeTopo, availRes)

		Convey("When aggregating resources", func() {
			expected := topologyv1alpha1.ZoneList{
				topologyv1alpha1.Zone{
					Name: "node-0",
					Type: "Node",
					Costs: topologyv1alpha1.CostList{
						topologyv1alpha1.CostInfo{
							Name:  "node-0",
							Value: 10,
						},
						topologyv1alpha1.CostInfo{
							Name:  "node-1",
							Value: 20,
						},
					},
					Resources: topologyv1alpha1.ResourceInfoList{
						topologyv1alpha1.ResourceInfo{
							Name:        "cpu",
							Available:   resource.MustParse("11"),
							Allocatable: resource.MustParse("11"),
							Capacity:    resource.MustParse("12"),
						},
						topologyv1alpha1.ResourceInfo{
							Name:        "fake.io/net",
							Available:   resource.MustParse("4"),
							Allocatable: resource.MustParse("4"),
							Capacity:    resource.MustParse("4"),
						},
					},
				},
				topologyv1alpha1.Zone{
					Name: "node-1",
					Type: "Node",
					Costs: topologyv1alpha1.CostList{
						topologyv1alpha1.CostInfo{
							Name:  "node-0",
							Value: 20,
						},
						topologyv1alpha1.CostInfo{
							Name:  "node-1",
							Value: 10,
						},
					},
					Resources: topologyv1alpha1.ResourceInfoList{
						topologyv1alpha1.ResourceInfo{
							Name:        "cpu",
							Available:   resource.MustParse("11"),
							Allocatable: resource.MustParse("11"),
							Capacity:    resource.MustParse("12"),
						},
						topologyv1alpha1.ResourceInfo{
							Name:        "fake.io/gpu",
							Available:   resource.MustParse("1"),
							Allocatable: resource.MustParse("1"),
							Capacity:    resource.MustParse("1"),
						},
						topologyv1alpha1.ResourceInfo{
							Name:        "fake.io/net",
							Available:   resource.MustParse("4"),
							Allocatable: resource.MustParse("4"),
							Capacity:    resource.MustParse("4"),
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
			log.Printf("result=%v", res)
			log.Printf("expected=%v", expected)
			log.Printf("diff=%s", cmp.Diff(res, expected))
			So(cmp.Equal(res, expected), ShouldBeFalse)
		})
	})

	Convey("When I aggregate the node resources fake data and some pod allocation", t, func() {
		availRes := &v1.AllocatableResourcesResponse{
			Devices: []*v1.ContainerDevices{
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netAAA"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 0,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/net",
					DeviceIds:    []string{"netBBB"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
								ID: 1,
							},
						},
					},
				},
				&v1.ContainerDevices{
					ResourceName: "fake.io/gpu",
					DeviceIds:    []string{"gpuAAA"},
					Topology: &v1.TopologyInfo{
						Nodes: []*v1.NUMANode{
							&v1.NUMANode{
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
		}

		resAggr = NewResourcesAggregatorFromData(&fakeTopo, availRes)

		Convey("When aggregating resources", func() {
			podRes := []PodResources{
				PodResources{
					Name:      "test-pod-0",
					Namespace: "default",
					Containers: []ContainerResources{
						ContainerResources{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								ResourceInfo{
									Name: "cpu",
									Data: []string{"5", "7"},
								},
								ResourceInfo{
									Name: "fake.io/net",
									Data: []string{"netBBB"},
								},
							},
						},
					},
				},
			}

			expected := topologyv1alpha1.ZoneList{
				topologyv1alpha1.Zone{
					Name: "node-0",
					Type: "Node",
					Costs: topologyv1alpha1.CostList{
						topologyv1alpha1.CostInfo{
							Name:  "node-0",
							Value: 10,
						},
						topologyv1alpha1.CostInfo{
							Name:  "node-1",
							Value: 20,
						},
					},
					Resources: topologyv1alpha1.ResourceInfoList{
						topologyv1alpha1.ResourceInfo{
							Name:        "cpu",
							Available:   resource.MustParse("11"),
							Allocatable: resource.MustParse("11"),
							Capacity:    resource.MustParse("12"),
						},
						topologyv1alpha1.ResourceInfo{
							Name:        "fake.io/net",
							Available:   resource.MustParse("1"),
							Allocatable: resource.MustParse("1"),
							Capacity:    resource.MustParse("1"),
						},
					},
				},
				topologyv1alpha1.Zone{
					Name: "node-1",
					Type: "Node",
					Costs: topologyv1alpha1.CostList{
						topologyv1alpha1.CostInfo{
							Name:  "node-0",
							Value: 20,
						},
						topologyv1alpha1.CostInfo{
							Name:  "node-1",
							Value: 10,
						},
					},
					Resources: topologyv1alpha1.ResourceInfoList{
						topologyv1alpha1.ResourceInfo{
							Name:        "cpu",
							Available:   resource.MustParse("10"),
							Allocatable: resource.MustParse("12"),
							Capacity:    resource.MustParse("12"),
						},
						topologyv1alpha1.ResourceInfo{
							Name:        "fake.io/gpu",
							Available:   resource.MustParse("1"),
							Allocatable: resource.MustParse("1"),
							Capacity:    resource.MustParse("1"),
						},
						topologyv1alpha1.ResourceInfo{
							Name:        "fake.io/net",
							Available:   resource.MustParse("0"),
							Allocatable: resource.MustParse("1"),
							Capacity:    resource.MustParse("1"),
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
			log.Printf("result=%v", res)
			log.Printf("expected=%v", expected)
			log.Printf("diff=%s", cmp.Diff(res, expected))
			So(cmp.Equal(res, expected), ShouldBeTrue)
		})
	})

}

// ghwc topology -f json
var testTopology string = `{
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
