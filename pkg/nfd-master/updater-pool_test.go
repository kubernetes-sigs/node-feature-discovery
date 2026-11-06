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

package nfdmaster

import (
	"errors"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8sclient "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func newFakeupdaterPool(nfdMaster *nfdMaster) *updaterPool {
	return &updaterPool{
		nfdMaster: nfdMaster,
		wg:        sync.WaitGroup{},
	}
}

func TestUpdaterStart(t *testing.T) {
	fakeMaster := newFakeMaster()
	updaterPool := newFakeupdaterPool(fakeMaster)

	Convey("New node updater pool should report running=false", t, func() {
		So(updaterPool.running(), ShouldBeFalse)
	})

	Convey("When starting the node updater pool", t, func() {
		updaterPool.start(10)
		Convey("Running node updater pool should report running=true", func() {
			So(updaterPool.running(), ShouldBeTrue)
		})
		q := updaterPool.queue
		Convey("Node updater pool queue properties should change", func() {
			So(q, ShouldNotBeNil)
			So(q.ShuttingDown(), ShouldBeFalse)
		})

		updaterPool.start(10)
		Convey("Node updater pool queue should not change", func() {
			So(updaterPool.queue, ShouldEqual, q)
		})
	})
}

func TestNodeUpdaterStop(t *testing.T) {
	fakeMaster := newFakeMaster()
	updaterPool := newFakeupdaterPool(fakeMaster)

	updaterPool.start(10)
	Convey("Running node updater pool should report running=true", t, func() {
		So(updaterPool.running(), ShouldBeTrue)
	})

	Convey("When stoping the node updater pool", t, func() {
		updaterPool.stop()
		Convey("Stopped node updater pool should report running=false", func() {
			So(updaterPool.running(), ShouldBeFalse)
		})
		Convey("Node updater pool queue should be removed", func() {
			// Wait for the wg.Done()
			So(func() any {
				return updaterPool.queue.ShuttingDown()
			}, withTimeout, 2*time.Second, ShouldBeTrue)
		})
	})
}

func TestRunNodeUpdater(t *testing.T) {
	fakeMaster := newFakeMaster(WithKubernetesClient(fakek8sclient.NewClientset()))
	//nolint:staticcheck
	fakeMaster.nfdController = newFakeNfdAPIController(fakenfdclient.NewSimpleClientset())
	updaterPool := newFakeupdaterPool(fakeMaster)

	updaterPool.start(10)
	Convey("Queue has no element", t, func() {
		So(updaterPool.queue.Len(), ShouldEqual, 0)
	})
	updaterPool.queue.Add(testNodeName)
	Convey("Added element to the queue should be removed", t, func() {
		So(func() any { return updaterPool.queue.Len() },
			withTimeout, 2*time.Second, ShouldEqual, 0)
	})
}

func TestRunNodeFeatureGroupUpdater(t *testing.T) {
	fakeMaster := newFakeMaster(WithKubernetesClient(fakek8sclient.NewClientset()))
	//nolint:staticcheck
	fakeMaster.nfdController = newFakeNfdAPIController(fakenfdclient.NewSimpleClientset())
	updaterPool := newFakeupdaterPool(fakeMaster)

	updaterPool.start(10)
	Convey("Queue has no element", t, func() {
		So(updaterPool.nfgQueue.Len(), ShouldEqual, 0)
	})
	updaterPool.nfgQueue.Add(testNodeName)
	Convey("Added element to the queue should be removed", t, func() {
		So(func() any { return updaterPool.queue.Len() },
			withTimeout, 2*time.Second, ShouldEqual, 0)
	})
}

func TestProcessNodeUpdateRequest(t *testing.T) {
	Convey("When processing a node update request", t, func() {
		fakeCli := fakek8sclient.NewClientset()
		fakeMaster := newFakeMaster(WithKubernetesClient(fakeCli))
		//nolint:staticcheck
		fakeMaster.nfdController = newFakeNfdAPIController(fakenfdclient.NewSimpleClientset())
		updaterPool := newFakeupdaterPool(fakeMaster)
		updaterPool.queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
		defer updaterPool.queue.ShutDown()
		updaterPool.queue.Add(testNodeName)

		Convey("When the node does not exist, it is not queued for retry", func() {
			So(updaterPool.processNodeUpdateRequest(fakeCli), ShouldBeTrue)
			So(updaterPool.queue.NumRequeues(testNodeName), ShouldEqual, 0)
		})

		Convey("When getting the node fails with an error other than NotFound, it is queued for retry", func() {
			fakeCli.PrependReactor("get", "nodes", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, apierrors.NewForbidden(corev1.Resource("nodes"), testNodeName, errors.New("fake error"))
			})
			So(updaterPool.processNodeUpdateRequest(fakeCli), ShouldBeTrue)
			So(updaterPool.queue.NumRequeues(testNodeName), ShouldEqual, 1)
		})
	})
}

func TestProcessNodeFeatureGroupUpdateRequestGetError(t *testing.T) {
	Convey("When getting the NodeFeatureGroup fails with an error other than NotFound, it is queued for retry", t, func() {
		//nolint:staticcheck
		nfdCli := fakenfdclient.NewSimpleClientset()
		nfdCli.PrependReactor("get", "nodefeaturegroups", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, apierrors.NewForbidden(nfdv1alpha1.SchemeGroupVersion.WithResource("nodefeaturegroups").GroupResource(), "nfg", errors.New("fake error"))
		})
		fakeMaster := newFakeMaster(WithKubernetesClient(fakek8sclient.NewClientset(newTestNode())), withNFDClient(nfdCli))
		fakeMaster.nfdController = newFakeNfdAPIController(nfdCli)
		updaterPool := newFakeupdaterPool(fakeMaster)
		updaterPool.nfgQueue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
		defer updaterPool.nfgQueue.ShutDown()
		key := "default/nfg"
		updaterPool.nfgQueue.Add(key)

		So(updaterPool.processNodeFeatureGroupUpdateRequest(nfdCli), ShouldBeTrue)
		So(updaterPool.nfgQueue.NumRequeues(key), ShouldEqual, 1)
	})
}
