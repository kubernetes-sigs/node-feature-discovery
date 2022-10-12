/*
Copyright 2021-2022 The Kubernetes Authors.

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
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	nfdscheme "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned/scheme"
	nfdinformers "sigs.k8s.io/node-feature-discovery/pkg/generated/informers/externalversions"
	nfdlisters "sigs.k8s.io/node-feature-discovery/pkg/generated/listers/nfd/v1alpha1"
)

type nfdController struct {
	featureLister nfdlisters.NodeFeatureLister
	ruleLister    nfdlisters.NodeFeatureRuleLister

	stopChan chan struct{}

	updateAllNodesChan chan struct{}
	updateOneNodeChan  chan string
}

func newNfdController(config *restclient.Config, disableNodeFeature bool) (*nfdController, error) {
	c := &nfdController{
		stopChan:           make(chan struct{}, 1),
		updateAllNodesChan: make(chan struct{}, 1),
		updateOneNodeChan:  make(chan string),
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)

	informerFactory := nfdinformers.NewSharedInformerFactory(nfdClient, 5*time.Minute)

	// Add informer for NodeFeature objects
	if !disableNodeFeature {
		featureInformer := informerFactory.Nfd().V1alpha1().NodeFeatures()
		if _, err := featureInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				klog.V(2).Infof("NodeFeature %v added", key)
				c.updateOneNode(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(newObj)
				klog.V(2).Infof("NodeFeature %v updated", key)
				c.updateOneNode(newObj)
			},
			DeleteFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				klog.V(2).Infof("NodeFeature %v deleted", key)
				c.updateOneNode(obj)
			},
		}); err != nil {
			return nil, err
		}
		c.featureLister = featureInformer.Lister()
	}

	// Add informer for NodeFeatureRule objects
	ruleInformer := informerFactory.Nfd().V1alpha1().NodeFeatureRules()
	if _, err := ruleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(object)
			klog.V(2).Infof("NodeFeatureRule %v added", key)
			if !disableNodeFeature {
				c.updateAllNodes()
			}
			// else: rules will be processed only when gRPC requests are received
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(newObject)
			klog.V(2).Infof("NodeFeatureRule %v updated", key)
			if !disableNodeFeature {
				c.updateAllNodes()
			}
			// else: rules will be processed only when gRPC requests are received
		},
		DeleteFunc: func(object interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(object)
			klog.V(2).Infof("NodeFeatureRule %v deleted", key)
			if !disableNodeFeature {
				c.updateAllNodes()
			}
			// else: rules will be processed only when gRPC requests are received
		},
	}); err != nil {
		return nil, err
	}
	c.ruleLister = ruleInformer.Lister()

	// Start informers
	informerFactory.Start(c.stopChan)

	utilruntime.Must(nfdv1alpha1.AddToScheme(nfdscheme.Scheme))

	return c, nil
}

func (c *nfdController) stop() {
	select {
	case c.stopChan <- struct{}{}:
	default:
	}
}

func (c *nfdController) updateOneNode(obj interface{}) {
	o, ok := obj.(*nfdv1alpha1.NodeFeature)
	if !ok {
		klog.Errorf("not a NodeFeature object (but of type %T): %v", obj, obj)
		return
	}

	nodeName, ok := o.Labels[nfdv1alpha1.NodeFeatureObjNodeNameLabel]
	if !ok {
		klog.Errorf("no node name for NodeFeature object %s/%s: %q label is missing",
			o.Namespace, o.Name, nfdv1alpha1.NodeFeatureObjNodeNameLabel)
		return
	}
	if nodeName == "" {
		klog.Errorf("no node name for NodeFeature object %s/%s: %q label is empty",
			o.Namespace, o.Name, nfdv1alpha1.NodeFeatureObjNodeNameLabel)
		return
	}

	c.updateOneNodeChan <- nodeName
}

func (c *nfdController) updateAllNodes() {
	select {
	case c.updateAllNodesChan <- struct{}{}:
	default:
	}
}
