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

package nfdtopologygarbagecollector

import (
	"context"
	"time"

	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
)

// Args are the command line arguments
type Args struct {
	GCPeriod time.Duration

	Kubeconfig string
}

type TopologyGC interface {
	Run() error
	Stop()
}

type topologyGC struct {
	stopChan   chan struct{}
	topoClient topologyclientset.Interface
	gcPeriod   time.Duration
	factory    informers.SharedInformerFactory
}

func New(args *Args) (TopologyGC, error) {
	kubeconfig, err := apihelper.GetKubeconfig(args.Kubeconfig)
	if err != nil {
		return nil, err
	}

	stop := make(chan struct{})

	return newTopologyGC(kubeconfig, stop, args.GCPeriod)
}

func newTopologyGC(config *restclient.Config, stop chan struct{}, gcPeriod time.Duration) (*topologyGC, error) {
	helper := apihelper.K8sHelpers{Kubeconfig: config}
	cli, err := helper.GetTopologyClient()
	if err != nil {
		return nil, err
	}

	clientset := kubernetes.NewForConfigOrDie(config)
	factory := informers.NewSharedInformerFactory(clientset, 5*time.Minute)

	return &topologyGC{
		topoClient: cli,
		stopChan:   stop,
		gcPeriod:   gcPeriod,
		factory:    factory,
	}, nil
}

func (n *topologyGC) deleteNRT(nodeName string) {
	if err := n.topoClient.TopologyV1alpha2().NodeResourceTopologies().Delete(context.TODO(), nodeName, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("NodeResourceTopology not found, omitting deletion", "nodeName", nodeName)
			return
		} else {
			klog.ErrorS(err, "failed to delete NodeResourceTopology object", "nodeName", nodeName)
			return
		}
	}
	klog.InfoS("NodeResourceTopology object has been deleted", "nodeName", nodeName)
}

func (n *topologyGC) deleteNodeHandler(object interface{}) {
	// handle a case when we are starting up and need to clear stale NRT resources
	obj := object
	if deletedFinalStateUnknown, ok := object.(cache.DeletedFinalStateUnknown); ok {
		klog.V(2).InfoS("found stale NodeResourceTopology object", "object", object)
		obj = deletedFinalStateUnknown.Obj
	}

	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.InfoS("cannot convert object to v1.Node", "object", object)
		return
	}

	n.deleteNRT(node.GetName())
}

func (n *topologyGC) runGC() {
	klog.InfoS("Running GC")
	objects := n.factory.Core().V1().Nodes().Informer().GetIndexer().List()
	nodes := sets.NewString()
	for _, object := range objects {
		key, err := cache.MetaNamespaceKeyFunc(object)
		if err != nil {
			klog.ErrorS(err, "failed to create key", "object", object)
			continue
		}
		nodes.Insert(key)
	}

	nrts, err := n.topoClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to list NodeResourceTopology objects")
		return
	}

	for _, nrt := range nrts.Items {
		key, err := cache.MetaNamespaceKeyFunc(&nrt)
		if err != nil {
			klog.ErrorS(err, "failed to create key", "noderesourcetopology", klog.KObj(&nrt))
			continue
		}
		if !nodes.Has(key) {
			n.deleteNRT(key)
		}
	}
}

// periodicGC runs garbage collector at every gcPeriod to make sure we haven't missed any node
func (n *topologyGC) periodicGC(gcPeriod time.Duration) {
	gcTrigger := time.NewTicker(gcPeriod)
	for {
		select {
		case <-gcTrigger.C:
			n.runGC()
		case <-n.stopChan:
			klog.InfoS("shutting down periodic Garbage Collector")
			return
		}
	}
}

func (n *topologyGC) run() error {
	nodeInformer := n.factory.Core().V1().Nodes().Informer()

	if _, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: n.deleteNodeHandler,
	}); err != nil {
		return err
	}

	// start informers
	n.factory.Start(n.stopChan)
	n.factory.WaitForCacheSync(n.stopChan)

	n.runGC()

	return nil
}

// Run is a blocking function that removes stale NRT objects when Node is deleted and runs periodic GC to make sure any obsolete objects are removed
func (n *topologyGC) Run() error {
	if err := n.run(); err != nil {
		return err
	}
	// run periodic GC
	n.periodicGC(n.gcPeriod)

	return nil
}

func (n *topologyGC) Stop() {
	select {
	case n.stopChan <- struct{}{}:
	default:
	}
}
