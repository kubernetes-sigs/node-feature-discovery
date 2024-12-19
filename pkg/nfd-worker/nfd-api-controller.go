/*
Copyright 2024 The Kubernetes Authors.

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

package nfdworker

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdscheme "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/scheme"
	nfdinformers "sigs.k8s.io/node-feature-discovery/api/generated/informers/externalversions"
	nfdinformersv1alpha1 "sigs.k8s.io/node-feature-discovery/api/generated/informers/externalversions/nfd/v1alpha1"
	nfdlisters "sigs.k8s.io/node-feature-discovery/api/generated/listers/nfd/v1alpha1"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

type nfdController struct {
	configLister     nfdlisters.NodeFeatureWorkerConfigLister
	stopChan         chan struct{}
	updateConfigChan chan nfdv1alpha1.NodeFeatureWorkerConfig
}

type nfdApiControllerOptions struct {
	ResyncPeriod time.Duration
}

func init() {
	utilruntime.Must(nfdv1alpha1.AddToScheme(nfdscheme.Scheme))
}

func newNfdController(config *restclient.Config, nfdApiControllerOptions nfdApiControllerOptions, ns string) (*nfdController, error) {
	c := &nfdController{
		stopChan:         make(chan struct{}),
		updateConfigChan: make(chan nfdv1alpha1.NodeFeatureWorkerConfig),
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)

	klog.V(2).InfoS("initializing new NFD API controller", "options", utils.DelayedDumper(nfdApiControllerOptions))

	informerFactory := nfdinformers.NewSharedInformerFactory(nfdClient, nfdApiControllerOptions.ResyncPeriod)

	// Add informer for NodeFeature objects
	tweakListOpts := func(opts *metav1.ListOptions) {
		// Tweak list opts on initial sync to avoid timeouts on the apiserver.
		// NodeFeature objects are huge and the Kubernetes apiserver
		// (v1.30) experiences http handler timeouts when the resource
		// version is set to some non-empty value (TODO: find out why).
		if opts.ResourceVersion == "0" {
			opts.ResourceVersion = ""
		}
	}
	configInformer := nfdinformersv1alpha1.New(informerFactory, ns, tweakListOpts).NodeFeatureWorkerConfigs()
	if _, err := configInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cfg := obj.(*nfdv1alpha1.NodeFeatureWorkerConfig)
			klog.V(2).InfoS("NodeFeatureWorkerConfig added", "workerconfig", klog.KObj(cfg))
			c.updateConfiguration(cfg)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cfg := newObj.(*nfdv1alpha1.NodeFeatureWorkerConfig)
			klog.V(2).InfoS("NodeFeatureWorkerConfig updated", "workerconfig", klog.KObj(cfg))
			c.updateConfiguration(cfg)
		},
		DeleteFunc: func(obj interface{}) {
			cfg := obj.(*nfdv1alpha1.NodeFeatureWorkerConfig)
			klog.V(2).InfoS("NodeFeatureWorkerConfig deleted", "workerconfig", klog.KObj(cfg))
			c.updateConfiguration(cfg)
		},
	}); err != nil {
		return nil, err
	}
	c.configLister = configInformer.Lister()

	// Start informers
	informerFactory.Start(c.stopChan)
	now := time.Now()
	ret := informerFactory.WaitForCacheSync(c.stopChan)
	for res, ok := range ret {
		if !ok {
			return nil, fmt.Errorf("informer cache failed to sync resource %s", res)
		}
	}

	klog.InfoS("informer caches synced", "duration", time.Since(now))

	return c, nil
}

func (c *nfdController) stop() {
	close(c.stopChan)
}

func (c *nfdController) updateConfiguration(cfg *nfdv1alpha1.NodeFeatureWorkerConfig) {
	select {
	case c.updateConfigChan <- *cfg:
	case <-c.stopChan:
	}
}
