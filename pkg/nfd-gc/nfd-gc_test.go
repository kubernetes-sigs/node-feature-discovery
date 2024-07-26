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

	topologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	metadataclient "k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/fake"
	fakemetadataclient "k8s.io/client-go/metadata/fake"
	"k8s.io/client-go/metadata/metadatainformer"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNRTGC(t *testing.T) {
	Convey("When theres is old NRT ", t, func() {
		gc := newMockGC(nil, []string{"node1"})

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		So(waitForNRT(gc.client), ShouldBeTrue)

		gc.Stop()
		So(<-errChan, ShouldBeNil)
	})
	Convey("When theres is one old NRT and one up to date", t, func() {
		gc := newMockGC([]string{"node1"}, []string{"node1", "node2"})

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		So(waitForNRT(gc.client, "node1"), ShouldBeTrue)

		gc.Stop()
		So(<-errChan, ShouldBeNil)
	})
	Convey("Should react to delete event", t, func() {
		gc := newMockGC([]string{"node1", "node2"}, []string{"node1", "node2"})

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		gvr := corev1.SchemeGroupVersion.WithResource("nodes")
		err := gc.client.Resource(gvr).Delete(context.TODO(), "node1", metav1.DeleteOptions{})
		So(err, ShouldBeNil)

		So(waitForNRT(gc.client, "node2"), ShouldBeTrue)
	})
	Convey("periodic GC should remove obsolete NRT", t, func() {
		gc := newMockGC([]string{"node1", "node2"}, []string{"node1", "node2"})
		// Override period to run fast
		gc.args.GCPeriod = 100 * time.Millisecond

		nrt := createPartialObjectMetadata("topology.node.k8s.io/v1alpha2", "NodeResourceTopology", "", "not-existing")

		errChan := make(chan error)
		go func() { errChan <- gc.Run() }()

		gvr := topologyv1alpha2.SchemeGroupVersion.WithResource("noderesourcetopologies")
		_, err := gc.client.Resource(gvr).(fake.MetadataClient).CreateFake(nrt, metav1.CreateOptions{})
		So(err, ShouldBeNil)

		So(waitForNRT(gc.client, "node1", "node2"), ShouldBeTrue)
	})
}

func newMockGC(nodes, nrts []string) *mockGC {
	// Create fake objects
	objs := []runtime.Object{}
	for _, name := range nodes {
		objs = append(objs, createPartialObjectMetadata("v1", "Node", "", name))
	}
	for _, name := range nrts {
		objs = append(objs, createPartialObjectMetadata("topology.node.k8s.io/v1alpha2", "NodeResourceTopology", "", name))
	}

	scheme := fake.NewTestScheme()
	_ = metav1.AddMetaToScheme(scheme)
	cli := fakemetadataclient.NewSimpleMetadataClient(scheme, objs...)
	return &mockGC{
		nfdGarbageCollector: nfdGarbageCollector{
			factory:  metadatainformer.NewSharedInformerFactory(cli, 0),
			client:   cli,
			stopChan: make(chan struct{}),
			args: &Args{
				GCPeriod: 10 * time.Minute,
			},
		},
		client: cli,
	}
}

func createPartialObjectMetadata(apiVersion, kind, namespace, name string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

type mockGC struct {
	nfdGarbageCollector

	client metadataclient.Interface
}

func waitForNRT(cli metadataclient.Interface, names ...string) bool {
	nameSet := sets.NewString(names...)
	gvr := topologyv1alpha2.SchemeGroupVersion.WithResource("noderesourcetopologies")
	for i := 0; i < 2; i++ {
		rsp, err := cli.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
		So(err, ShouldBeNil)

		nrtNames := sets.NewString()
		for _, meta := range rsp.Items {
			nrtNames.Insert(meta.Name)
		}

		if nrtNames.Equal(nameSet) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}
