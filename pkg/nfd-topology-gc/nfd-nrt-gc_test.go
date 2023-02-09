/*
Copyright 2023 The Kubernetes Authors.

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

package nfdtopologygarbagecollector

import (
	"context"
	"testing"
	"time"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	faketopologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	fakek8sclientset "k8s.io/client-go/kubernetes/fake"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNRTGC(t *testing.T) {
	Convey("When theres is old NRT ", t, func() {
		k8sClient := fakek8sclientset.NewSimpleClientset()

		fakeClient := faketopologyv1alpha2.NewSimpleClientset(&v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
		})
		factory := informers.NewSharedInformerFactory(k8sClient, 5*time.Minute)

		stopChan := make(chan struct{}, 1)

		gc := &topologyGC{
			factory:    factory,
			topoClient: fakeClient,
			stopChan:   stopChan,
			gcPeriod:   10 * time.Minute,
		}

		err := gc.run()
		So(err, ShouldBeNil)

		nrts, err := fakeClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)
		So(nrts.Items, ShouldHaveLength, 0)

		gc.Stop()
	})
	Convey("When theres is one old NRT and one up to date", t, func() {
		k8sClient := fakek8sclientset.NewSimpleClientset(&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
		})

		fakeClient := faketopologyv1alpha2.NewSimpleClientset(&v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
		},
			&v1alpha2.NodeResourceTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		)

		stopChan := make(chan struct{}, 1)

		factory := informers.NewSharedInformerFactory(k8sClient, 5*time.Minute)

		gc := &topologyGC{
			factory:    factory,
			topoClient: fakeClient,
			stopChan:   stopChan,
			gcPeriod:   10 * time.Minute,
		}

		err := gc.run()
		So(err, ShouldBeNil)

		nrts, err := fakeClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)
		So(nrts.Items, ShouldHaveLength, 1)
		So(nrts.Items[0].GetName(), ShouldEqual, "node1")

	})
	Convey("Should react to delete event", t, func() {
		k8sClient := fakek8sclientset.NewSimpleClientset(
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
			},
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		)

		fakeClient := faketopologyv1alpha2.NewSimpleClientset(
			&v1alpha2.NodeResourceTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
			},
			&v1alpha2.NodeResourceTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		)

		stopChan := make(chan struct{}, 1)

		factory := informers.NewSharedInformerFactory(k8sClient, 5*time.Minute)
		gc := &topologyGC{
			factory:    factory,
			topoClient: fakeClient,
			stopChan:   stopChan,
			gcPeriod:   10 * time.Minute,
		}

		err := gc.run()
		So(err, ShouldBeNil)

		nrts, err := fakeClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)

		So(nrts.Items, ShouldHaveLength, 2)

		err = k8sClient.CoreV1().Nodes().Delete(context.TODO(), "node1", metav1.DeleteOptions{})
		So(err, ShouldBeNil)
		// simple sleep with retry loop to make sure indexer will pick up event and trigger deleteNode Function
		deleted := false
		for i := 0; i < 5; i++ {
			nrts, err := fakeClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
			So(err, ShouldBeNil)

			if len(nrts.Items) == 1 {
				deleted = true
				break
			}
			time.Sleep(time.Second)
		}
		So(deleted, ShouldBeTrue)
	})
	Convey("periodic GC should remove obsolete NRT", t, func() {
		k8sClient := fakek8sclientset.NewSimpleClientset(
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
			},
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		)

		fakeClient := faketopologyv1alpha2.NewSimpleClientset(
			&v1alpha2.NodeResourceTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
			},
			&v1alpha2.NodeResourceTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		)

		stopChan := make(chan struct{}, 1)

		factory := informers.NewSharedInformerFactory(k8sClient, 5*time.Minute)
		gc := &topologyGC{
			factory:    factory,
			topoClient: fakeClient,
			stopChan:   stopChan,
			gcPeriod:   time.Second,
		}

		err := gc.run()
		So(err, ShouldBeNil)

		nrts, err := fakeClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)

		So(nrts.Items, ShouldHaveLength, 2)

		nrt := v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name: "not-existing",
			},
		}

		go gc.periodicGC(time.Second)

		_, err = fakeClient.TopologyV1alpha2().NodeResourceTopologies().Create(context.TODO(), &nrt, metav1.CreateOptions{})
		So(err, ShouldBeNil)
		// simple sleep with retry loop to make sure GC was triggered
		deleted := false
		for i := 0; i < 5; i++ {
			nrts, err := fakeClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
			So(err, ShouldBeNil)

			if len(nrts.Items) == 2 {
				deleted = true
				break
			}
			time.Sleep(2 * time.Second)
		}
		So(deleted, ShouldBeTrue)
	})

}
