/*
Copyright 2021 The Kubernetes Authors.

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
	lister nfdlisters.NodeFeatureRuleLister

	stopChan chan struct{}
}

func newNfdController(config *restclient.Config) *nfdController {
	c := &nfdController{
		stopChan: make(chan struct{}, 1),
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)

	informerFactory := nfdinformers.NewSharedInformerFactory(nfdClient, 5*time.Minute)
	informer := informerFactory.Nfd().V1alpha1().NodeFeatureRules()
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(object)
			klog.V(2).Infof("LabelRule %v added", key)
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(newObject)
			klog.V(2).Infof("LabelRule %v updated", key)
		},
		DeleteFunc: func(object interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(object)
			klog.V(2).Infof("LabelRule %v deleted", key)
		},
	})
	informerFactory.Start(c.stopChan)

	utilruntime.Must(nfdv1alpha1.AddToScheme(nfdscheme.Scheme))

	c.lister = informer.Lister()

	return c
}

func (c *nfdController) stop() {
	select {
	case c.stopChan <- struct{}{}:
	default:
	}
}
