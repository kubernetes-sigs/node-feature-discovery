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

package nfdgarbagecollector

import (
	"context"
	"time"

	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// Args are the command line arguments
type Args struct {
	GCPeriod    time.Duration
	Kubeconfig  string
	MetricsPort int
}

type NfdGarbageCollector interface {
	Run() error
	Stop()
}

type nfdGarbageCollector struct {
	args       *Args
	stopChan   chan struct{}
	nfdClient  nfdclientset.Interface
	topoClient topologyclientset.Interface
	factory    informers.SharedInformerFactory
}

func New(args *Args) (NfdGarbageCollector, error) {
	kubeconfig, err := utils.GetKubeconfig(args.Kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset := kubernetes.NewForConfigOrDie(kubeconfig)

	return &nfdGarbageCollector{
		args:       args,
		stopChan:   make(chan struct{}),
		topoClient: topologyclientset.NewForConfigOrDie(kubeconfig),
		nfdClient:  nfdclientset.NewForConfigOrDie(kubeconfig),
		factory:    informers.NewSharedInformerFactory(clientset, 5*time.Minute),
	}, nil
}

func (n *nfdGarbageCollector) deleteNodeFeature(namespace, name string) {
	kind := "NodeFeature"
	if err := n.nfdClient.NfdV1alpha1().NodeFeatures(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("NodeFeature not found, omitting deletion", "nodefeature", klog.KRef(namespace, name))
			return
		} else {
			klog.ErrorS(err, "failed to delete NodeFeature object", "nodefeature", klog.KRef(namespace, name))
			objectDeleteErrors.WithLabelValues(kind).Inc()
			return
		}
	}
	klog.InfoS("NodeFeature object has been deleted", "nodefeature", klog.KRef(namespace, name))
	objectsDeleted.WithLabelValues(kind).Inc()
}

func (n *nfdGarbageCollector) deleteNRT(nodeName string) {
	kind := "NodeResourceTopology"
	if err := n.topoClient.TopologyV1alpha2().NodeResourceTopologies().Delete(context.TODO(), nodeName, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("NodeResourceTopology not found, omitting deletion", "nodeName", nodeName)
			return
		} else {
			klog.ErrorS(err, "failed to delete NodeResourceTopology object", "nodeName", nodeName)
			objectDeleteErrors.WithLabelValues(kind).Inc()
			return
		}
	}
	klog.InfoS("NodeResourceTopology object has been deleted", "nodeName", nodeName)
	objectsDeleted.WithLabelValues(kind).Inc()
}

func (n *nfdGarbageCollector) deleteNodeHandler(object interface{}) {
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

	// Delete all NodeFeature objects (from all namespaces) targeting the deleted node
	nfListOptions := metav1.ListOptions{LabelSelector: nfdv1alpha1.NodeFeatureObjNodeNameLabel + "=" + node.GetName()}
	if nfs, err := n.nfdClient.NfdV1alpha1().NodeFeatures("").List(context.TODO(), nfListOptions); err != nil {
		klog.ErrorS(err, "failed to list NodeFeature objects")
	} else {
		for _, nf := range nfs.Items {
			n.deleteNodeFeature(nf.Namespace, nf.Name)
		}
	}
}

// garbageCollect removes all stale API objects
func (n *nfdGarbageCollector) garbageCollect() {
	klog.InfoS("performing garbage collection")
	nodes, err := n.factory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "failed to list Node objects")
		return
	}
	nodeNames := sets.NewString()
	for _, node := range nodes {
		nodeNames.Insert(node.Name)
	}

	// Handle NodeFeature objects
	nfs, err := n.nfdClient.NfdV1alpha1().NodeFeatures("").List(context.TODO(), metav1.ListOptions{})
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("NodeFeature CRD does not exist")
	} else if err != nil {
		klog.ErrorS(err, "failed to list NodeFeature objects")
	} else {
		for _, nf := range nfs.Items {
			nodeName, ok := nf.GetLabels()[nfdv1alpha1.NodeFeatureObjNodeNameLabel]
			if !ok {
				klog.InfoS("node name label missing from NodeFeature object", "nodefeature", klog.KObj(&nf))
			}
			if !nodeNames.Has(nodeName) {
				n.deleteNodeFeature(nf.Namespace, nf.Name)
			}
		}
	}

	// Handle NodeResourceTopology objects
	nrts, err := n.topoClient.TopologyV1alpha2().NodeResourceTopologies().List(context.TODO(), metav1.ListOptions{})
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("NodeResourceTopology CRD does not exist")
	} else if err != nil {
		klog.ErrorS(err, "failed to list NodeResourceTopology objects")
	} else {
		for _, nrt := range nrts.Items {
			if !nodeNames.Has(nrt.Name) {
				n.deleteNRT(nrt.Name)
			}
		}
	}
}

// periodicGC runs garbage collector at every gcPeriod to make sure we haven't missed any node
func (n *nfdGarbageCollector) periodicGC(gcPeriod time.Duration) {
	// Do initial round of garbage collection at startup time
	n.garbageCollect()

	gcTrigger := time.NewTicker(gcPeriod)
	defer gcTrigger.Stop()
	for {
		select {
		case <-gcTrigger.C:
			n.garbageCollect()
		case <-n.stopChan:
			klog.InfoS("shutting down periodic Garbage Collector")
			return
		}
	}
}

func (n *nfdGarbageCollector) startNodeInformer() error {
	nodeInformer := n.factory.Core().V1().Nodes().Informer()

	if _, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: n.deleteNodeHandler,
	}); err != nil {
		return err
	}

	// start informers
	n.factory.Start(n.stopChan)
	n.factory.WaitForCacheSync(n.stopChan)

	return nil
}

// Run is a blocking function that removes stale NRT objects when Node is deleted and runs periodic GC to make sure any obsolete objects are removed
func (n *nfdGarbageCollector) Run() error {
	if n.args.MetricsPort > 0 {
		m := utils.CreateMetricsServer(n.args.MetricsPort,
			buildInfo,
			objectsDeleted,
			objectDeleteErrors)
		go m.Run()
		registerVersion(version.Get())
		defer m.Stop()
	}

	if err := n.startNodeInformer(); err != nil {
		return err
	}
	// run periodic GC
	n.periodicGC(n.args.GCPeriod)

	return nil
}

func (n *nfdGarbageCollector) Stop() {
	close(n.stopChan)
}
