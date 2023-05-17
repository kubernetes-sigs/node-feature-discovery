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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	nfdscheme "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned/scheme"
	nfdinformers "sigs.k8s.io/node-feature-discovery/pkg/generated/informers/externalversions"
	nfdlisters "sigs.k8s.io/node-feature-discovery/pkg/generated/listers/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

type nfdController struct {
	featureLister nfdlisters.NodeFeatureLister
	ruleLister    nfdlisters.NodeFeatureRuleLister

	stopChan chan struct{}

	updateAllNodesChan chan struct{}
	updateOneNodeChan  chan string
}

type nfdApiControllerOptions struct {
	DisableNodeFeature bool
	ResyncPeriod       time.Duration
}

func newNfdController(config *restclient.Config, nfdApiControllerOptions nfdApiControllerOptions) (*nfdController, error) {
	c := &nfdController{
		stopChan:           make(chan struct{}, 1),
		updateAllNodesChan: make(chan struct{}, 1),
		updateOneNodeChan:  make(chan string),
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)
	klog.V(2).InfoS("initializing new NFD API controller", "options", utils.DelayedDumper(nfdApiControllerOptions))

	informerFactory := nfdinformers.NewSharedInformerFactory(nfdClient, nfdApiControllerOptions.ResyncPeriod)

	// Add informer for NodeFeature objects
	if !nfdApiControllerOptions.DisableNodeFeature {
		featureInformer := informerFactory.Nfd().V1alpha1().NodeFeatures()
		if _, err := featureInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				nfr := obj.(*nfdv1alpha1.NodeFeature)
				klog.V(2).InfoS("NodeFeature added", "nodefeature", klog.KObj(nfr))
				c.updateOneNode("NodeFeature", nfr)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				nfr := newObj.(*nfdv1alpha1.NodeFeature)
				klog.V(2).InfoS("NodeFeature updated", "nodefeature", klog.KObj(nfr))
				c.updateOneNode("NodeFeature", nfr)
			},
			DeleteFunc: func(obj interface{}) {
				nfr := obj.(*nfdv1alpha1.NodeFeature)
				klog.V(2).InfoS("NodeFeature deleted", "nodefeature", klog.KObj(nfr))
				c.updateOneNode("NodeFeature", nfr)
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
			klog.V(2).InfoS("NodeFeatureRule added", "nodefeaturerule", klog.KObj(object.(metav1.Object)))
			if !nfdApiControllerOptions.DisableNodeFeature {
				c.updateAllNodes()
			}
			// else: rules will be processed only when gRPC requests are received
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			klog.V(2).InfoS("NodeFeatureRule updated", "nodefeaturerule", klog.KObj(newObject.(metav1.Object)))
			if !nfdApiControllerOptions.DisableNodeFeature {
				c.updateAllNodes()
			}
			// else: rules will be processed only when gRPC requests are received
		},
		DeleteFunc: func(object interface{}) {
			klog.V(2).InfoS("NodeFeatureRule deleted", "nodefeaturerule", klog.KObj(object.(metav1.Object)))
			if !nfdApiControllerOptions.DisableNodeFeature {
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

func (c *nfdController) updateOneNode(typ string, obj metav1.Object) {
	nodeName, err := getNodeNameForObj(obj)
	if err != nil {
		klog.ErrorS(err, "failed to determine node name for object", "type", typ, "object", klog.KObj(obj))
		return
	}
	c.updateOneNodeChan <- nodeName
}

func getNodeNameForObj(obj metav1.Object) (string, error) {
	nodeName, ok := obj.GetLabels()[nfdv1alpha1.NodeFeatureObjNodeNameLabel]
	if !ok {
		return "", fmt.Errorf("%q label is missing", nfdv1alpha1.NodeFeatureObjNodeNameLabel)
	}
	if nodeName == "" {
		return "", fmt.Errorf("%q label is empty", nfdv1alpha1.NodeFeatureObjNodeNameLabel)
	}
	return nodeName, nil
}

func (c *nfdController) updateAllNodes() {
	select {
	case c.updateAllNodesChan <- struct{}{}:
	default:
	}
}
