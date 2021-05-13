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

package resourcemonitor

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"

	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"

	"sigs.k8s.io/node-feature-discovery/pkg/podres"
)

func TestPodScanner(t *testing.T) {

	var resScan ResourcesScanner
	var err error

	Convey("When I scan for pod resources using fake client and no namespace", t, func() {
		mockPodResClient := new(podres.MockPodResourcesListerClient)
		resScan, err = NewPodResourcesScanner("", mockPodResClient)

		Convey("Creating a Resources Scanner using a mock client", func() {
			So(err, ShouldBeNil)
		})

		Convey("When I get error", func() {
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(nil, fmt.Errorf("fake error"))
			res, err := resScan.Scan()

			Convey("Error is present", func() {
				So(err, ShouldNotBeNil)
			})
			Convey("Return PodResources should be nil", func() {
				So(res, ShouldBeNil)
			})
		})

		Convey("When I successfully get empty response", func() {
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(&v1.ListPodResourcesResponse{}, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should be zero", func() {
				So(len(res), ShouldEqual, 0)
			})
		})

		Convey("When I successfully get valid response", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					&v1.PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							&v1.ContainerResources{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									&v1.ContainerDevices{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
										Topology: &v1.TopologyInfo{
											Nodes: []*v1.NUMANode{
												&v1.NUMANode{ID: 0},
											},
										},
									},
								},
								CpuIds: []int64{0, 1},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							ContainerResources{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									ResourceInfo{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
									ResourceInfo{
										Name: "fake.io/resource",
										Data: []string{"devA"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res, expected), ShouldBeTrue)
			})
		})

		Convey("When I successfully get valid response without topology", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					&v1.PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							&v1.ContainerResources{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									&v1.ContainerDevices{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
									},
								},
								CpuIds: []int64{0, 1},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							ContainerResources{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									ResourceInfo{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
									ResourceInfo{
										Name: "fake.io/resource",
										Data: []string{"devA"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res, expected), ShouldBeTrue)
			})
		})

		Convey("When I successfully get valid response without devices", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					&v1.PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							&v1.ContainerResources{
								Name:   "test-cnt-0",
								CpuIds: []int64{0, 1},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							ContainerResources{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									ResourceInfo{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res, expected), ShouldBeTrue)
			})
		})

		Convey("When I successfully get valid response without cpus", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					&v1.PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							&v1.ContainerResources{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									&v1.ContainerDevices{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
									},
								},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							ContainerResources{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									ResourceInfo{
										Name: "fake.io/resource",
										Data: []string{"devA"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res, expected), ShouldBeTrue)
			})
		})

		Convey("When I successfully get valid response without resources", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					&v1.PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							&v1.ContainerResources{
								Name: "test-cnt-0",
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should not have values", func() {
				So(len(res), ShouldEqual, 0)
			})
		})

	})

	Convey("When I scan for pod resources using fake client and given namespace", t, func() {
		mockPodResClient := new(podres.MockPodResourcesListerClient)
		resScan, err = NewPodResourcesScanner("pod-res-test", mockPodResClient)

		Convey("Creating a Resources Scanner using a mock client", func() {
			So(err, ShouldBeNil)
		})

		Convey("When I get error", func() {
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(nil, fmt.Errorf("fake error"))
			res, err := resScan.Scan()

			Convey("Error is present", func() {
				So(err, ShouldNotBeNil)
			})
			Convey("Return PodResources should be nil", func() {
				So(res, ShouldBeNil)
			})
		})

		Convey("When I successfully get empty response", func() {
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(&v1.ListPodResourcesResponse{}, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should be zero", func() {
				So(len(res), ShouldEqual, 0)
			})
		})

		Convey("When I successfully get valid response", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					&v1.PodResources{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							&v1.ContainerResources{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									&v1.ContainerDevices{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
										Topology: &v1.TopologyInfo{
											Nodes: []*v1.NUMANode{
												&v1.NUMANode{ID: 0},
											},
										},
									},
								},
								CpuIds: []int64{0, 1},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should be zero", func() {
				So(len(res), ShouldEqual, 0)
			})
		})

	})

}
