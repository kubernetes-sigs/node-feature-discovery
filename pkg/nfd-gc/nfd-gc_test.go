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

package nfdgarbagecollector

import (
	"context"
	"testing"
	"time"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	faketopologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	k8sclientset "k8s.io/client-go/kubernetes"
	fakek8sclientset "k8s.io/client-go/kubernetes/fake"
	fakenfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNRTGC(t *testing.T) {
	Convey("When theres is old NRT ", t, func() {
		gc := newMockGC(nil, []string{"node1"}, []string{"pod1"})

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		So(waitForNRT(gc.topoClient), ShouldBeTrue)

		gc.Stop()
		So(<-errChan, ShouldBeNil)
	})
	Convey("When theres is one old NRT and one up to date", t, func() {
		gc := newMockGC([]string{"node1"}, []string{"node1", "node2"}, []string{"pod1", "pod2"})

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		So(waitForNRT(gc.topoClient, "node1"), ShouldBeTrue)

		gc.Stop()
		So(<-errChan, ShouldBeNil)
	})
	Convey("Should react to delete event", t, func() {
		gc := newMockGC([]string{"node1", "node2"}, []string{"node1", "node2"}, []string{"pod1", "pod2"})

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		err := gc.k8sClient.CoreV1().Nodes().Delete(context.TODO(), "node1", metav1.DeleteOptions{})
		So(err, ShouldBeNil)

		So(waitForNRT(gc.topoClient, "node2"), ShouldBeTrue)
	})
	Convey("periodic GC should remove obsolete NRT", t, func() {
		gc := newMockGC([]string{"node1", "node2"}, []string{"node1", "node2"}, []string{"pod1", "pod2"})
		// Override period to run fast
		gc.args.GCPeriod = 100 * time.Millisecond

		nrt := v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name: "not-existing",
			},
		}

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		_, err := gc.topoClient.TopologyV1alpha2().NodeResourceTopologies().Create(context.TODO(), &nrt, metav1.CreateOptions{})
		So(err, ShouldBeNil)

		So(waitForNRT(gc.topoClient, "node1", "node2"), ShouldBeTrue)
	})
	Convey("periodic GC should remove stale NRT", t, func() {
		gc := newMockGC([]string{"node1", "node2"}, []string{"node1", "node2"}, []string{"pod1", "pod2"})
		// Override period to run fast
		gc.args.GCPeriod = 100 * time.Millisecond

		nrt := v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node3",
				Labels: map[string]string{"owner-pod": "pod4"},
			},
		}

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		_, err := gc.topoClient.TopologyV1alpha2().NodeResourceTopologies().Create(context.TODO(), &nrt, metav1.CreateOptions{})
		So(err, ShouldBeNil)

		So(waitForNrtPodsGC(gc.topoClient, "pod1", "pod2"), ShouldBeTrue)
	})
}

func newMockGC(nodes, nrts, pods []string) *mockGC {
	k8sClient := fakek8sclientset.NewSimpleClientset(createFakeNodes(nodes...)...)
	return &mockGC{
		nfdGarbageCollector: nfdGarbageCollector{
			factory:    informers.NewSharedInformerFactory(k8sClient, 5*time.Minute),
			nfdClient:  fakenfdclientset.NewSimpleClientset(),
			topoClient: faketopologyv1alpha2.NewSimpleClientset(createFakeNRTs(nrts, pods)...),
			k8sClient:  fakek8sclientset.NewSimpleClientset(createFakePods(pods...)...),
			stopChan:   make(chan struct{}),
			args: &Args{
				GCPeriod: 10 * time.Minute,
			},
		},
		k8sClient: k8sClient,
	}
}

func createFakeNodes(names ...string) []runtime.Object {
	nodes := make([]runtime.Object, len(names))
	for i, n := range names {
		nodes[i] = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: n,
			}}
	}
	return nodes
}

func createFakePods(names ...string) []runtime.Object {
	pods := make([]runtime.Object, len(names))
	for i, n := range names {
		pods[i] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: n,
			}}
	}
	return pods
}

func createFakeNRTs(names, pods []string) []runtime.Object {
	nrts := make([]runtime.Object, len(names))
	for i, n := range names {
		nrts[i] = &v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name:   n,
				Labels: map[string]string{"owner-pod": pods[i]},
			}}
	}
	return nrts
}

type mockGC struct {
	nfdGarbageCollector

	k8sClient k8sclientset.Interface
}

func waitForNRT(cli topologyclientset.Interface, names ...string) bool {
	nameSet := sets.NewString(names...)
	for i := 0; i < 2; i++ {
		nrts, err := cli.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)

		nrtNames := sets.NewString()
		for _, nrt := range nrts.Items {
			nrtNames.Insert(nrt.Name)
		}

		if nrtNames.Equal(nameSet) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func waitForNrtPodsGC(cli topologyclientset.Interface, pods ...string) bool {
	podsSet := sets.NewString(pods...)
	for i := 0; i < 2; i++ {
		nrts, err := cli.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)

		nrtPods := sets.NewString()
		for _, nrt := range nrts.Items {
			nrtPods.Insert(nrt.Labels["owner-pod"])
		}

		if nrtPods.Equal(podsSet) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}
