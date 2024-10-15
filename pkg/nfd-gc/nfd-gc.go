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
	"fmt"
	"strings"
	"time"

	topologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	metadataclient "k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

var (
	gvrNF   = nfdv1alpha1.SchemeGroupVersion.WithResource("nodefeatures")
	gvrNRT  = topologyv1alpha2.SchemeGroupVersion.WithResource("noderesourcetopologies")
	gvrNode = corev1.SchemeGroupVersion.WithResource("nodes")
	gvrPod  = corev1.SchemeGroupVersion.WithResource("pods")
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
	args     *Args
	stopChan chan struct{}
	client   metadataclient.Interface
	factory  metadatainformer.SharedInformerFactory
	// gcNRTs holds owner-pod and namespace information for detecting stale NRT objects
	gcNRTs map[string]string
}

func New(args *Args) (NfdGarbageCollector, error) {
	kubeconfig, err := utils.GetKubeconfig(args.Kubeconfig)
	if err != nil {
		return nil, err
	}

	cli := metadataclient.NewForConfigOrDie(kubeconfig)

	return &nfdGarbageCollector{
		args:     args,
		stopChan: make(chan struct{}),
		client:   cli,
		factory:  metadatainformer.NewSharedInformerFactory(cli, 0),
		gcNRTs:   make(map[string]string),
	}, nil
}

func (n *nfdGarbageCollector) deleteNodeFeature(namespace, name string) {
	kind := "NodeFeature"
	if err := n.client.Resource(gvrNF).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
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
	if err := n.client.Resource(gvrNRT).Delete(context.TODO(), nodeName, metav1.DeleteOptions{}); err != nil {
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

	meta, ok := obj.(*metav1.PartialObjectMetadata)
	if !ok {
		klog.InfoS("cannot convert object to metav1.ObjectMeta", "object", object)
		return
	}
	nodeName := meta.ObjectMeta.GetName()

	n.deleteNRT(nodeName)

	// Delete all NodeFeature objects (from all namespaces) targeting the deleted node
	nfListOptions := metav1.ListOptions{LabelSelector: nfdv1alpha1.NodeFeatureObjNodeNameLabel + "=" + nodeName}
	if nfs, err := n.client.Resource(gvrNF).List(context.TODO(), nfListOptions); err != nil {
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
	objs, err := n.factory.ForResource(gvrNode).Lister().List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "failed to list Node objects")
		return
	}
	nodeNames := sets.NewString()
	for _, obj := range objs {
		meta := obj.(*metav1.PartialObjectMetadata).ObjectMeta
		nodeNames.Insert(meta.Name)
	}

	listAndHandle := func(gvr schema.GroupVersionResource, handler func(metav1.PartialObjectMetadata)) {
		opts := metav1.ListOptions{
			Limit: 200,
		}
		for {
			rsp, err := n.client.Resource(gvr).List(context.TODO(), opts)
			if errors.IsNotFound(err) {
				klog.V(2).InfoS("resource does not exist", "resource", gvr)
				break
			} else if err != nil {
				klog.ErrorS(err, "failed to list objects", "resource", gvr)
				break
			}
			for _, item := range rsp.Items {
				handler(item)
			}

			if rsp.ListMeta.Continue == "" {
				break
			}
			opts.Continue = rsp.ListMeta.Continue
		}
	}

	// Handle NodeFeature objects
	listAndHandle(gvrNF, func(meta metav1.PartialObjectMetadata) {
		nodeName, ok := meta.GetLabels()[nfdv1alpha1.NodeFeatureObjNodeNameLabel]
		if !ok {
			klog.InfoS("node name label missing from NodeFeature object", "nodefeature", klog.KObj(&meta))
		}
		if !nodeNames.Has(nodeName) {
			n.deleteNodeFeature(meta.Namespace, meta.Name)
		}
	})

	// Handle NodeResourceTopology objects
	listAndHandle(gvrNRT, func(meta metav1.PartialObjectMetadata) {
		deleteNRT := false

		if objRef, ok := meta.Annotations[nfdtopologyupdater.NRTOwnerPodAnnotation]; ok {
			if s := strings.Split(objRef, "/"); len(s) == 2 {
				pod := s[1]
				_, err := n.client.Resource(gvrPod).Get(context.TODO(), pod, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					if val, ok := n.gcNRTs[meta.Name]; !ok {
						n.gcNRTs[meta.Name] = objRef
					} else {
						if val != objRef {
							n.gcNRTs[meta.Name] = objRef
						} else {
							delete(n.gcNRTs, meta.Name)
							deleteNRT = true
						}
					}
				} else if err != nil {
					klog.ErrorS(err, "failed to get Pod object")
				}
			}
		}

		if !nodeNames.Has(meta.Name) || deleteNRT {
			n.deleteNRT(meta.Name)
		}
	})
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
	nodeInformer := n.factory.ForResource(gvrNode).Informer()

	if _, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: n.deleteNodeHandler,
	}); err != nil {
		return err
	}

	// start informers
	n.factory.Start(n.stopChan)

	start := time.Now()
	ret := n.factory.WaitForCacheSync(n.stopChan)
	for res, ok := range ret {
		if !ok {
			return fmt.Errorf("node informer cache failed to sync (%s)", res)
		}
	}
	klog.InfoS("node informer cache synced", "duration", time.Since(start))

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
