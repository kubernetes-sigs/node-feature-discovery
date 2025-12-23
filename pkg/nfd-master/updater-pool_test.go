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
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	fakek8sclient "k8s.io/client-go/kubernetes/fake"
	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
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
			So(func() interface{} {
				return updaterPool.queue.ShuttingDown()
			}, withTimeout, 2*time.Second, ShouldBeTrue)
		})
	})
}

func TestRunNodeUpdater(t *testing.T) {
	//nolint:staticcheck // See issue #2400 for migration to NewClientset
	fakeMaster := newFakeMaster(WithKubernetesClient(fakek8sclient.NewSimpleClientset()))
	fakeMaster.nfdController = newFakeNfdAPIController(fakenfdclient.NewSimpleClientset())
	updaterPool := newFakeupdaterPool(fakeMaster)

	updaterPool.start(10)
	Convey("Queue has no element", t, func() {
		So(updaterPool.queue.Len(), ShouldEqual, 0)
	})
	updaterPool.queue.Add(testNodeName)
	Convey("Added element to the queue should be removed", t, func() {
		So(func() interface{} { return updaterPool.queue.Len() },
			withTimeout, 2*time.Second, ShouldEqual, 0)
	})
}

func TestRunNodeFeatureGroupUpdater(t *testing.T) {
	//nolint:staticcheck // See issue #2400 for migration to NewClientset
	fakeMaster := newFakeMaster(WithKubernetesClient(fakek8sclient.NewSimpleClientset()))
	fakeMaster.nfdController = newFakeNfdAPIController(fakenfdclient.NewSimpleClientset())
	updaterPool := newFakeupdaterPool(fakeMaster)

	updaterPool.start(10)
	Convey("Queue has no element", t, func() {
		So(updaterPool.nfgQueue.Len(), ShouldEqual, 0)
	})
	updaterPool.nfgQueue.Add(testNodeName)
	Convey("Added element to the queue should be removed", t, func() {
		So(func() interface{} { return updaterPool.queue.Len() },
			withTimeout, 2*time.Second, ShouldEqual, 0)
	})
}
