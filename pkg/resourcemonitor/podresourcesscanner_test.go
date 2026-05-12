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
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/k8stopologyawareschedwg/podfingerprint"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"

	mockpodres "sigs.k8s.io/node-feature-discovery/pkg/podres/mocks"
)

// newFakePodLister returns a PodLister backed by an in-memory indexer
// pre-populated with the given pods. It avoids spinning up a fake
// clientset / informer / sync loop in unit tests; PodLister only needs
// an Indexer, and tests for PodResourcesScanner exercise lookups not
// reflector behavior.
func newFakePodLister(pods ...*corev1.Pod) corev1listers.PodLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range pods {
		_ = indexer.Add(pod)
	}
	return corev1listers.NewPodLister(indexer)
}

type errorPodLister struct {
	err error
}

func (l errorPodLister) List(selector labels.Selector) ([]*corev1.Pod, error) {
	return nil, l.err
}

func (l errorPodLister) Pods(namespace string) corev1listers.PodNamespaceLister {
	return errorPodNamespaceLister(l)
}

type errorPodNamespaceLister struct {
	err error
}

func (l errorPodNamespaceLister) List(selector labels.Selector) ([]*corev1.Pod, error) {
	return nil, l.err
}

func (l errorPodNamespaceLister) Get(name string) (*corev1.Pod, error) {
	return nil, l.err
}

func TestNewPodResourcesScannerRequiresKubernetesClient(t *testing.T) {
	Convey("When I create a pod resources scanner without a Kubernetes client", t, func() {
		resScan, err := NewPodResourcesScanner("*", nil, newFakePodLister(), nil, false)

		Convey("Error is present", func() {
			So(err, ShouldNotBeNil)
		})
		Convey("Return ResourcesScanner should be nil", func() {
			So(resScan, ShouldBeNil)
		})
	})
}

func TestIsWatchable(t *testing.T) {
	Convey("When pod is missing from the lister and live GET succeeds", t, func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test-container",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(100, resource.DecimalSI),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(100, resource.DecimalSI),
							},
						},
					},
				},
			},
		}
		resScan := &PodResourcesScanner{
			namespace: "*",
			podLister: newFakePodLister(),
			k8sClient: fakeclient.NewClientset(pod),
		}

		isWatchable, isExclusiveCPUs, err := resScan.isWatchable(context.Background(), &v1.PodResources{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		})

		Convey("Error is nil", func() {
			So(err, ShouldBeNil)
		})
		Convey("Pod is watchable", func() {
			So(isWatchable, ShouldBeTrue)
		})
		Convey("Pod has exclusive CPUs", func() {
			So(isExclusiveCPUs, ShouldBeTrue)
		})
	})

	Convey("When pod is missing from the lister and live GET fails", t, func() {
		resScan := &PodResourcesScanner{
			namespace: "*",
			podLister: newFakePodLister(),
			k8sClient: fakeclient.NewClientset(),
		}

		_, _, err := resScan.isWatchable(context.Background(), &v1.PodResources{
			Name:      "missing-pod",
			Namespace: "default",
		})

		Convey("NotFound error is returned", func() {
			So(apierrors.IsNotFound(err), ShouldBeTrue)
		})
	})

	Convey("When pod lister returns a non-NotFound error", t, func() {
		listerErr := errors.New("lister failure")
		resScan := &PodResourcesScanner{
			namespace: "*",
			podLister: errorPodLister{err: listerErr},
			k8sClient: fakeclient.NewClientset(),
		}

		_, _, err := resScan.isWatchable(context.Background(), &v1.PodResources{
			Name:      "test-pod",
			Namespace: "default",
		})

		Convey("Lister error is returned", func() {
			So(errors.Is(err, listerErr), ShouldBeTrue)
		})
	})
}

func TestPodScanner(t *testing.T) {
	// PodFingerprint only depends on Name/Namespace of the pods running on a Node
	// so we can precalculate the expected value
	expectedFingerprintCompute := func(pods []*corev1.Pod) (string, error) {
		pf := podfingerprint.NewFingerprint(len(pods))
		for _, pr := range pods {
			if err := pf.Add(pr.Namespace, pr.Name); err != nil {
				return "", err
			}
		}
		return pf.Sign(), nil
	}

	Convey("When I scan for pod resources using fake client and no namespace", t, func() {
		mockPodResClient := new(mockpodres.PodResourcesListerClient)

		computePodFingerprint := true
		resScan, err := NewPodResourcesScanner("*", mockPodResClient, newFakePodLister(), fakeclient.NewClientset(), computePodFingerprint)

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
				So(res.PodResources, ShouldBeNil)
			})
			Convey("Return Attributes should be empty", func() {
				So(res.Attributes, ShouldBeEmpty)
			})
		})

		Convey("When I successfully get empty response", func() {
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(&v1.ListPodResourcesResponse{}, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should be zero", func() {
				So(len(res.PodResources), ShouldEqual, 0)
			})
			Convey("Return Attributes should be empty", func() {
				So(res.Attributes, ShouldBeEmpty)
			})
		})

		Convey("When I successfully get valid response", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
										Topology: &v1.TopologyInfo{
											Nodes: []*v1.NUMANode{
												{ID: 0},
											},
										},
									},
								},
								CpuIds: []int64{0, 1},
								Memory: []*v1.ContainerMemory{
									{
										MemoryType: "hugepages-2Mi",
										Size:       512,
										Topology: &v1.TopologyInfo{
											Nodes: []*v1.NUMANode{
												{ID: 1},
											},
										},
									},
									{
										MemoryType: "memory",
										Size:       512,
										Topology: &v1.TopologyInfo{
											Nodes: []*v1.NUMANode{
												{ID: 0},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(512, resource.DecimalSI),
									"hugepages-2Mi":                         *resource.NewQuantity(512, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(512, resource.DecimalSI),
									"hugepages-2Mi":                         *resource.NewQuantity(512, resource.DecimalSI),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}

			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
									{
										Name:        "fake.io/resource",
										Data:        []string{"devA"},
										NumaNodeIds: []int{0},
									},
									{
										Name:        "hugepages-2Mi",
										Data:        []string{"512"},
										NumaNodeIds: []int{1},
									},
									{
										Name:        "memory",
										Data:        []string{"512"},
										NumaNodeIds: []int{0},
									},
								},
							},
						},
					},
				}
				for _, podresource := range res.PodResources {
					for _, container := range podresource.Containers {
						sort.Slice(res.PodResources, func(i, j int) bool {
							return container.Resources[i].Name < container.Resources[j].Name
						})
					}
				}
				So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
			})
			Convey("Return Attributes should have pod fingerprint attribute with proper value", func() {
				So(len(res.Attributes), ShouldEqual, 1)
				// can compute expected fringerprint only with the list of pods in the node.
				expectedFingerprint, err := expectedFingerprintCompute([]*corev1.Pod{pod})
				So(err, ShouldBeNil)
				So(res.Attributes[0].Name, ShouldEqual, podfingerprint.Attribute)
				So(res.Attributes[0].Value, ShouldEqual, expectedFingerprint)
			})
		})

		Convey("When I successfully get valid response without topology", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-cnt-0",
							Image:           "nginx",
							ImagePullPolicy: "Always",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}

			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
									{
										Name: "fake.io/resource",
										Data: []string{"devA"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
			})
			Convey("Return Attributes should have pod fingerprint attribute with proper value", func() {
				So(len(res.Attributes), ShouldEqual, 1)
				// can compute expected fringerprint only with the list of pods in the node.
				expectedFingerprint, err := expectedFingerprintCompute([]*corev1.Pod{pod})
				So(err, ShouldBeNil)
				So(res.Attributes[0].Name, ShouldEqual, podfingerprint.Attribute)
				So(res.Attributes[0].Value, ShouldEqual, expectedFingerprint)
			})
		})

		Convey("When I successfully get valid response without devices", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name:   "test-cnt-0",
								CpuIds: []int64{0, 1},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
			})
			Convey("Return Attributes should have pod fingerprint attribute with proper value", func() {
				So(len(res.Attributes), ShouldEqual, 1)
				// can compute expected fringerprint only with the list of pods in the node.
				expectedFingerprint, err := expectedFingerprintCompute([]*corev1.Pod{pod})
				So(err, ShouldBeNil)
				So(res.Attributes[0].Name, ShouldEqual, podfingerprint.Attribute)
				So(res.Attributes[0].Value, ShouldEqual, expectedFingerprint)
			})
		})

		Convey("When I successfully get valid response without cpus", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-cnt-0",
							Image:           "nginx",
							ImagePullPolicy: "Always",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []ContainerResources{
							{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									{
										Name: "fake.io/resource",
										Data: []string{"devA"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
			})
		})

		Convey("When I successfully get valid response for (non-guaranteed) pods with devices without cpus", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
								},
							},
						},
					},
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)
			})
			Convey("Return Attributes should have pod fingerprint attribute with proper value", func() {
				So(len(res.Attributes), ShouldEqual, 1)
				// can compute expected fringerprint only with the list of pods in the node.
				expectedFingerprint, err := expectedFingerprintCompute([]*corev1.Pod{pod})
				So(err, ShouldBeNil)
				So(res.Attributes[0].Name, ShouldEqual, podfingerprint.Attribute)
				So(res.Attributes[0].Value, ShouldEqual, expectedFingerprint)
			})

			expected := []PodResources{
				{
					Name:      "test-pod-0",
					Namespace: "default",
					Containers: []ContainerResources{
						{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								{
									Name: "fake.io/resource",
									Data: []string{"devA"},
								},
							},
						},
					},
				},
			}

			So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
		})

		Convey("When I successfully get valid response for (non-guaranteed) pods with devices with cpus", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name:   "test-cnt-0",
								CpuIds: []int64{0, 1},
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{

									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("1500m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("1500m"),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)
			})

			expected := []PodResources{
				{
					Name:      "test-pod-0",
					Namespace: "default",
					Containers: []ContainerResources{
						{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								{
									Name: "fake.io/resource",
									Data: []string{"devA"},
								},
							},
						},
					},
				},
			}
			So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)

			Convey("Return Attributes should have pod fingerprint attribute with proper value", func() {
				So(len(res.Attributes), ShouldEqual, 1)

				// can compute expected fringerprint only with the list of pods in the node.
				expectedFingerprint, err := expectedFingerprintCompute([]*corev1.Pod{pod})
				So(err, ShouldBeNil)
				So(res.Attributes[0].Name, ShouldEqual, podfingerprint.Attribute)
				So(res.Attributes[0].Value, ShouldEqual, expectedFingerprint)
			})
		})
	})

	Convey("When I scan for pod resources using fake client and given namespace", t, func() {
		mockPodResClient := new(mockpodres.PodResourcesListerClient)
		computePodFingerprint := false
		resScan, err := NewPodResourcesScanner("pod-res-test", mockPodResClient, newFakePodLister(), fakeclient.NewClientset(), computePodFingerprint)

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
				So(res.PodResources, ShouldBeNil)
			})
		})

		Convey("When I successfully get empty response", func() {
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(&v1.ListPodResourcesResponse{}, nil)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should be zero", func() {
				So(len(res.PodResources), ShouldEqual, 0)
			})
		})

		Convey("When I successfully get valid response", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
										Topology: &v1.TopologyInfo{
											Nodes: []*v1.NUMANode{
												{ID: 0},
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should be zero", func() {
				So(len(res.PodResources), ShouldEqual, 0)
			})
		})

		Convey("When I successfully get valid response when pod is in the monitoring namespace", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "pod-res-test",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-cnt-0",
							Image:           "nginx",
							ImagePullPolicy: "Always",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})

			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)

				expected := []PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "pod-res-test",
						Containers: []ContainerResources{
							{
								Name: "test-cnt-0",
								Resources: []ResourceInfo{
									{
										Name: "cpu",
										Data: []string{"0", "1"},
									},
									{
										Name: "fake.io/resource",
										Data: []string{"devA"},
									},
								},
							},
						},
					},
				}

				So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
			})
		})

		Convey("When I successfully get valid empty response for a pod not in the monitoring namespace without devices", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name:   "test-cnt-0",
								CpuIds: []int64{0, 1},
							},
						},
					},
				},
			}
			mockPodResClient.On("List", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*v1.ListPodResourcesRequest")).Return(resp, nil)
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				Convey("Return PodResources should be zero", func() {
					So(len(res.PodResources), ShouldEqual, 0)
				})
			})
		})

		Convey("When I successfully get an empty valid response for a pod without cpus when pod is not in the monitoring namespace", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "default",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-cnt-0",
							Image:           "nginx",
							ImagePullPolicy: "Always",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:                      *resource.NewQuantity(2, resource.DecimalSI),
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
								},
							},
						},
					},
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldEqual, 0)
			})
		})

		Convey("When I successfully get valid response for (non-guaranteed) pods with devices without cpus", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "pod-res-test",
						Containers: []*v1.ContainerResources{
							{
								Name: "test-cnt-0",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)
			})

			expected := []PodResources{
				{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
					Containers: []ContainerResources{
						{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								{
									Name: "fake.io/resource",
									Data: []string{"devA"},
								},
							},
						},
					},
				},
			}

			So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
		})

		Convey("When I successfully get valid response for (non-guaranteed) pods with devices with cpus", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "pod-res-test",
						Containers: []*v1.ContainerResources{
							{
								Name:   "test-cnt-0",
								CpuIds: []int64{0, 1},
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{

									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("1500m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("1500m"),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)
			})

			expected := []PodResources{
				{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
					Containers: []ContainerResources{
						{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								{
									Name: "fake.io/resource",
									Data: []string{"devA"},
								},
							},
						},
					},
				},
			}
			So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
		})

		Convey("When I successfully get valid response for guaranteed pods with not cpu pin containers", func() {
			resp := &v1.ListPodResourcesResponse{
				PodResources: []*v1.PodResources{
					{
						Name:      "test-pod-0",
						Namespace: "pod-res-test",
						Containers: []*v1.ContainerResources{
							{
								Name:   "test-cnt-0",
								CpuIds: []int64{0, 1},
								Devices: []*v1.ContainerDevices{
									{
										ResourceName: "fake.io/resource",
										DeviceIds:    []string{"devA"},
									},
								},
							},
							{
								Name: "test-cnt-1",
								Devices: []*v1.ContainerDevices{
									{
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
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-cnt-0",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{

									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("2"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("2"),
								},
							},
						},
						{
							Name: "test-cnt-1",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{

									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("1500m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceName("fake.io/resource"): *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory:                   *resource.NewQuantity(100, resource.DecimalSI),
									corev1.ResourceCPU:                      resource.MustParse("1500m"),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					QOSClass: corev1.PodQOSGuaranteed,
				},
			}
			resScan.(*PodResourcesScanner).podLister = newFakePodLister(pod)
			res, err := resScan.Scan()

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Return PodResources should have values", func() {
				So(len(res.PodResources), ShouldBeGreaterThan, 0)
			})

			expected := []PodResources{
				{
					Name:      "test-pod-0",
					Namespace: "pod-res-test",
					Containers: []ContainerResources{
						{
							Name: "test-cnt-0",
							Resources: []ResourceInfo{
								{
									Name: corev1.ResourceCPU,
									Data: []string{"0", "1"},
								},
								{
									Name: "fake.io/resource",
									Data: []string{"devA"},
								},
							},
						},
						{
							Name: "test-cnt-1",
							Resources: []ResourceInfo{
								{
									Name: "fake.io/resource",
									Data: []string{"devA"},
								},
							},
						},
					},
				},
			}
			So(reflect.DeepEqual(res.PodResources, expected), ShouldBeTrue)
		})

	})
}
