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
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type nodeFeatureGroupUpdaterPool struct {
	queue workqueue.RateLimitingInterface
	sync.Mutex

	wg        sync.WaitGroup
	nfdMaster *nfdMaster
}

func newNodeFeatureGroupUpdaterPool(nfdMaster *nfdMaster) *nodeFeatureGroupUpdaterPool {
	return &nodeFeatureGroupUpdaterPool{
		nfdMaster: nfdMaster,
		wg:        sync.WaitGroup{},
	}
}

func (u *nodeFeatureGroupUpdaterPool) processNodeFeatureGroupUpdateRequest(queue workqueue.RateLimitingInterface) bool {
	nodeName, quit := queue.Get()
	if quit {
		return false
	}

	defer queue.Done(nodeName)

	nodeFeatureGroupUpdateRequests.Inc()
	if err := u.nfdMaster.nfdAPIUpdateNodeFeatureGroup(nodeName.(string)); err != nil {
		if queue.NumRequeues(nodeName) < 15 {
			klog.InfoS("retrying NodeFeatureGroup update")
			queue.AddRateLimited(nodeName)
			return true
		} else {
			klog.ErrorS(err, "failed to update NodeFeatureGroup")
		}
	}
	queue.Forget(nodeName)
	return true
}

func (u *nodeFeatureGroupUpdaterPool) runNodeFeatureGroupUpdater(queue workqueue.RateLimitingInterface) {
	for u.processNodeFeatureGroupUpdateRequest(queue) {
	}
	u.wg.Done()
}

func (u *nodeFeatureGroupUpdaterPool) start(parallelism int) {
	u.Lock()
	defer u.Unlock()

	if u.queue != nil && !u.queue.ShuttingDown() {
		klog.InfoS("the NFD master NodeFeatureGroup updater pool is already running.")
		return
	}

	klog.InfoS("starting the NFD master NodeFeatureGroup updater pool", "parallelism", parallelism)

	// Create ratelimiter. Mimic workqueue.DefaultControllerRateLimiter() but
	// with modified per-item (node) rate limiting parameters.
	rl := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(50*time.Millisecond, 100*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
	u.queue = workqueue.NewRateLimitingQueue(rl)

	for i := 0; i < parallelism; i++ {
		u.wg.Add(1)
		go u.runNodeFeatureGroupUpdater(u.queue)
	}
}

func (u *nodeFeatureGroupUpdaterPool) stop() {
	u.Lock()
	defer u.Unlock()

	if u.queue == nil || u.queue.ShuttingDown() {
		klog.InfoS("the NFD master NodeFeatureGroup updater pool is not running.")
		return
	}

	klog.InfoS("stopping the NFD master NodeFeatureGroup updater pool")
	u.queue.ShutDown()
	u.wg.Wait()
}
