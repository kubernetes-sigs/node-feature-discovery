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
	"github.com/stretchr/testify/mock"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	"sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned/fake"
)

func newMockNodeUpdaterPool(nfdMaster *nfdMaster) *nodeUpdaterPool {
	return &nodeUpdaterPool{
		nfdMaster: nfdMaster,
		wg:        sync.WaitGroup{},
	}
}

func TestNodeUpdaterStart(t *testing.T) {
	mockHelper := &apihelper.MockAPIHelpers{}
	mockMaster := newMockMaster(mockHelper)
	mockNodeUpdaterPool := newMockNodeUpdaterPool(mockMaster)

	Convey("When starting the node updater pool", t, func() {
		mockNodeUpdaterPool.start(10)
		q := mockNodeUpdaterPool.queue
		Convey("Node updater pool queue properties should change", func() {
			So(q, ShouldNotBeNil)
			So(q.ShuttingDown(), ShouldBeFalse)
		})

		mockNodeUpdaterPool.start(10)
		Convey("Node updater pool queue should not change", func() {
			So(mockNodeUpdaterPool.queue, ShouldEqual, q)
		})
	})
}

func TestNodeUpdaterStop(t *testing.T) {
	mockHelper := &apihelper.MockAPIHelpers{}
	mockMaster := newMockMaster(mockHelper)
	mockNodeUpdaterPool := newMockNodeUpdaterPool(mockMaster)

	mockNodeUpdaterPool.start(10)

	Convey("When stoping the node updater pool", t, func() {
		mockNodeUpdaterPool.stop()
		Convey("Node updater pool queue should be removed", func() {
			// Wait for the wg.Done()
			So(func() interface{} {
				return mockNodeUpdaterPool.queue.ShuttingDown()
			}, withTimeout, 2*time.Second, ShouldBeTrue)
		})
	})
}

func TestRunNodeUpdater(t *testing.T) {
	mockAPIHelper := &apihelper.MockAPIHelpers{}
	mockMaster := newMockMaster(mockAPIHelper)
	mockMaster.nfdController = newMockNfdAPIController(fake.NewSimpleClientset())
	mockClient := &k8sclient.Clientset{}
	mockNode := newMockNode()
	mockNodeUpdaterPool := newMockNodeUpdaterPool(mockMaster)
	statusPatches := []apihelper.JsonPatch{}
	metadataPatches := []apihelper.JsonPatch{}

	mockAPIHelper.On("GetClient").Return(mockClient, nil)
	mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
	mockAPIHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(statusPatches))).Return(nil)
	mockAPIHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(metadataPatches))).Return(nil)

	mockNodeUpdaterPool.start(10)
	Convey("Queue has no element", t, func() {
		So(mockNodeUpdaterPool.queue.Len(), ShouldEqual, 0)
	})
	mockNodeUpdaterPool.queue.Add(mockNodeName)
	Convey("Added element to the queue should be removed", t, func() {
		So(func() interface{} { return mockNodeUpdaterPool.queue.Len() },
			withTimeout, 2*time.Second, ShouldEqual, 0)
	})
}
