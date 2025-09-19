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
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/features"
)

type updaterPool struct {
	queue    workqueue.TypedRateLimitingInterface[string]
	nfgQueue workqueue.TypedRateLimitingInterface[string]
	sync.RWMutex

	wg        sync.WaitGroup
	nfgWg     sync.WaitGroup
	nfdMaster *nfdMaster
}

func newUpdaterPool(nfdMaster *nfdMaster) *updaterPool {
	return &updaterPool{
		nfdMaster: nfdMaster,
		wg:        sync.WaitGroup{},
	}
}

func (u *updaterPool) processNodeUpdateRequest(cli k8sclient.Interface, queue workqueue.TypedRateLimitingInterface[string]) bool {
	nodeName, quit := queue.Get()
	if quit {
		return false
	}

	defer queue.Done(nodeName)

	nodeUpdateRequests.Inc()

	// Check if node exists
	if node, err := getNode(cli, nodeName); apierrors.IsNotFound(err) {
		klog.InfoS("node not found, skip update", "nodeName", nodeName)
	} else if err := u.nfdMaster.nfdAPIUpdateOneNode(cli, node); err != nil {
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

func (u *updaterPool) runNodeUpdater(queue workqueue.TypedRateLimitingInterface[string]) {
	var cli k8sclient.Interface
	if u.nfdMaster.kubeconfig != nil {
		// For normal execution, initialize a separate api client for each updater
		cli = k8sclient.NewForConfigOrDie(u.nfdMaster.kubeconfig)
	} else {
		// For tests, re-use the api client from nfd-master
		cli = u.nfdMaster.k8sClient
	}
	for u.processNodeUpdateRequest(cli, queue) {
	}
	u.wg.Done()
}

func (u *updaterPool) processNodeFeatureGroupUpdateRequest(cli nfdclientset.Interface, ngfQueue workqueue.TypedRateLimitingInterface[string]) bool {
	nfgName, quit := ngfQueue.Get()
	if quit {
		return false
	}
	defer ngfQueue.Done(nfgName)

	nodeFeatureGroupUpdateRequests.Inc()

	// Check if NodeFeatureGroup exists
	var nfg *nfdv1alpha1.NodeFeatureGroup
	var err error
	if nfg, err = getNodeFeatureGroup(cli, u.nfdMaster.namespace, nfgName); apierrors.IsNotFound(err) {
		klog.InfoS("NodeFeatureGroup not found, skip update", "NodeFeatureGroupName", nfgName)
	} else if err := u.nfdMaster.nfdAPIUpdateNodeFeatureGroup(u.nfdMaster.nfdClient, nfg); err != nil {
		if n := ngfQueue.NumRequeues(nfgName); n < 15 {
			klog.InfoS("retrying NodeFeatureGroup update", "nodeFeatureGroup", klog.KObj(nfg), "lastError", err)
		} else {
			klog.ErrorS(err, "failed to update NodeFeatureGroup, queueing for retry", "nodeFeatureGroup", klog.KObj(nfg), "lastError", err, "numRetries", n)
		}
		ngfQueue.AddRateLimited(nfgName)
		return true
	}

	ngfQueue.Forget(nfgName)
	return true
}

func (u *updaterPool) runNodeFeatureGroupUpdater(ngfQueue workqueue.TypedRateLimitingInterface[string]) {
	cli := nfdclientset.NewForConfigOrDie(u.nfdMaster.kubeconfig)
	for u.processNodeFeatureGroupUpdateRequest(cli, ngfQueue) {
	}
	u.nfgWg.Done()
}

func (u *updaterPool) start(parallelism int) {
	u.Lock()
	defer u.Unlock()

	if u.queue != nil && !u.queue.ShuttingDown() {
		klog.InfoS("the NFD master updater pool is already running.")
		return
	}

	if u.nfgQueue != nil && !u.nfgQueue.ShuttingDown() {
		klog.InfoS("the NFD master node feature group updater pool is already running.")
		return
	}

	klog.InfoS("starting the NFD master updater pool", "parallelism", parallelism)

	// Create ratelimiter. Mimic workqueue.DefaultControllerRateLimiter() but
	// with modified per-item (node) rate limiting parameters.
	rl := workqueue.NewTypedMaxOfRateLimiter[string](
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](50*time.Millisecond, 100*time.Second),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
	u.queue = workqueue.NewTypedRateLimitingQueue[string](rl)
	u.nfgQueue = workqueue.NewTypedRateLimitingQueue[string](rl)

	for i := 0; i < parallelism; i++ {
		u.wg.Add(1)
		go u.runNodeUpdater(u.queue)
		if features.NFDFeatureGate.Enabled(features.NodeFeatureGroupAPI) {
			u.nfgWg.Add(1)
			go u.runNodeFeatureGroupUpdater(u.nfgQueue)
		}
	}
}

func (u *updaterPool) stop() {
	u.Lock()
	defer u.Unlock()

	if u.queue == nil || u.queue.ShuttingDown() {
		klog.InfoS("the NFD master updater pool is not running.")
		return
	}

	if u.nfgQueue == nil || u.nfgQueue.ShuttingDown() {
		klog.InfoS("the NFD master updater pool is not running.")
		return
	}

	klog.InfoS("stopping the NFD master updater pool")
	u.queue.ShutDown()
	u.wg.Wait()
	u.nfgQueue.ShutDown()
	u.nfgWg.Wait()
}

func (u *updaterPool) addNode(nodeName string) {
	u.RLock()
	defer u.RUnlock()
	u.queue.Add(nodeName)
}

func (u *updaterPool) addNodeFeatureGroup(nodeFeatureGroupName string) {
	u.RLock()
	defer u.RUnlock()
	u.nfgQueue.Add(nodeFeatureGroupName)
}
