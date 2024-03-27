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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type nodeUpdaterPool struct {
	queue workqueue.RateLimitingInterface
	sync.RWMutex

	wg        sync.WaitGroup
	nfdMaster *nfdMaster
}

func newNodeUpdaterPool(nfdMaster *nfdMaster) *nodeUpdaterPool {
	return &nodeUpdaterPool{
		nfdMaster: nfdMaster,
		wg:        sync.WaitGroup{},
	}
}

func (u *nodeUpdaterPool) processNodeUpdateRequest(queue workqueue.RateLimitingInterface) bool {
	n, quit := queue.Get()
	if quit {
		return false
	}
	nodeName := n.(string)

	defer queue.Done(nodeName)

	nodeUpdateRequests.Inc()

	// Check if node exists
	if _, err := u.nfdMaster.getNode(nodeName); apierrors.IsNotFound(err) {
		klog.InfoS("node not found, skip update", "nodeName", nodeName)
	} else if err := u.nfdMaster.nfdAPIUpdateOneNode(nodeName); err != nil {
		if n := queue.NumRequeues(nodeName); n < 15 {
			klog.InfoS("retrying node update", "nodeName", nodeName, "lastError", err, "numRetries", n)
		} else {
			klog.ErrorS(err, "node update failed, queuing for retry ", "nodeName", nodeName, "numRetries", n)
			// Count only long-failing attempts
			nodeUpdateFailures.Inc()
		}
		queue.AddRateLimited(nodeName)
		return true
	}
	queue.Forget(nodeName)
	return true
}

func (u *nodeUpdaterPool) runNodeUpdater(queue workqueue.RateLimitingInterface) {
	for u.processNodeUpdateRequest(queue) {
	}
	u.wg.Done()
}

func (u *nodeUpdaterPool) start(parallelism int) {
	u.Lock()
	defer u.Unlock()

	if u.queue != nil && !u.queue.ShuttingDown() {
		klog.InfoS("the NFD master node updater pool is already running.")
		return
	}

	klog.InfoS("starting the NFD master node updater pool", "parallelism", parallelism)

	// Create ratelimiter. Mimic workqueue.DefaultControllerRateLimiter() but
	// with modified per-item (node) rate limiting parameters.
	rl := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(50*time.Millisecond, 100*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
	u.queue = workqueue.NewRateLimitingQueue(rl)

	for i := 0; i < parallelism; i++ {
		u.wg.Add(1)
		go u.runNodeUpdater(u.queue)
	}
}

func (u *nodeUpdaterPool) stop() {
	u.Lock()
	defer u.Unlock()

	if u.queue == nil || u.queue.ShuttingDown() {
		klog.InfoS("the NFD master node updater pool is not running.")
		return
	}

	klog.InfoS("stopping the NFD master node updater pool")
	u.queue.ShutDown()
	u.wg.Wait()
}

func (u *nodeUpdaterPool) addNode(nodeName string) {
	u.RLock()
	defer u.RUnlock()
	u.queue.Add(nodeName)
}
