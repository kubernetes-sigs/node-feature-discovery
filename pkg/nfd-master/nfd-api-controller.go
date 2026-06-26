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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sclient "k8s.io/client-go/kubernetes"
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
	featureLister      nfdlisters.NodeFeatureLister
	ruleLister         nfdlisters.NodeFeatureRuleLister
	featureGroupLister nfdlisters.NodeFeatureGroupLister

	stopChan chan struct{}

	updateAllNodesChan             chan struct{}
	updateOneNodeChan              chan string
	updateAllNodeFeatureGroupsChan chan struct{}
	updateNodeFeatureGroupChan     chan string

	namespaceLister *NamespaceLister

	// onDelete is called when a NodeFeature is deleted. This allows the
	// nfdMaster to track pending deletes and distinguish between "NodeFeature
	// was deleted" vs "NodeFeature is missing from incomplete cache".
	onDelete func(nodeName string)

	disableNodeFeatureGroup bool
}

type nfdApiControllerOptions struct {
	DisableNodeFeatureGroup      bool
	ResyncPeriod                 time.Duration
	K8sClient                    k8sclient.Interface
	NodeFeatureNamespaceSelector *metav1.LabelSelector
	ListSize                     int64
	// OnNodeFeatureDelete is called when a NodeFeature is deleted, before
	// queuing the node for update. This allows tracking pending deletes.
	OnNodeFeatureDelete func(nodeName string)
}

func init() {
	utilruntime.Must(nfdv1alpha1.AddToScheme(nfdscheme.Scheme))
}

// specChanged reports whether the reconcile-relevant part of an object — its
// .Spec — differs between the old and new versions delivered to an informer
// UpdateFunc. Metadata-only updates (annotations, labels, ownerReferences,
// resourceVersion), status-only updates, and resync no-ops (oldObj == newObj)
// therefore return false, letting the handler skip an unnecessary reconcile.
// It fails open (returns true) when the objects cannot be type-asserted, so a
// needed reconcile is never wrongly skipped.
func specChanged[T any](oldObj, newObj interface{}, specOf func(*T) any) bool {
	oldT, ok1 := oldObj.(*T)
	newT, ok2 := newObj.(*T)
	if !ok1 || !ok2 {
		return true
	}
	return !apiequality.Semantic.DeepEqual(specOf(oldT), specOf(newT))
}

func newNfdController(config *restclient.Config, nfdApiControllerOptions nfdApiControllerOptions) (*nfdController, error) {
	c := &nfdController{
		stopChan:                       make(chan struct{}),
		updateAllNodesChan:             make(chan struct{}, 1),
		updateOneNodeChan:              make(chan string),
		updateAllNodeFeatureGroupsChan: make(chan struct{}, 1),
		updateNodeFeatureGroupChan:     make(chan string),
		onDelete:                       nfdApiControllerOptions.OnNodeFeatureDelete,
		disableNodeFeatureGroup:        nfdApiControllerOptions.DisableNodeFeatureGroup,
	}

	if err := c.configureNamespaceLister(nfdApiControllerOptions); err != nil {
		return nil, err
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)
	klog.V(2).InfoS("initializing new NFD API controller", "options", utils.DelayedDumper(nfdApiControllerOptions))

	informerFactory := nfdinformers.NewSharedInformerFactory(nfdClient, nfdApiControllerOptions.ResyncPeriod)

	if err := c.addNodeFeatureInformer(informerFactory, nfdApiControllerOptions.ListSize); err != nil {
		return nil, err
	}
	if err := c.addNodeFeatureRuleInformer(informerFactory); err != nil {
		return nil, err
	}
	if !nfdApiControllerOptions.DisableNodeFeatureGroup {
		if err := c.addNodeFeatureGroupInformer(informerFactory); err != nil {
			return nil, err
		}
	}

	// Start informers
	informerFactory.Start(c.stopChan)
	now := time.Now()
	ret := informerFactory.WaitForCacheSync(c.stopChan)
	for res, ok := range ret {
		if !ok {
			// Stop the informers started above so their goroutines
			// don't leak when construction fails.
			c.stop()
			return nil, fmt.Errorf("informer cache failed to sync resource %s", res)
		}
	}

	klog.InfoS("informer caches synced", "duration", time.Since(now))

	return c, nil
}

// configureNamespaceLister sets up the namespace lister used to filter
// NodeFeature objects by namespace. It is a no-op when no namespace selector
// was configured, in which case all namespaces are allowed.
func (c *nfdController) configureNamespaceLister(opts nfdApiControllerOptions) error {
	if opts.NodeFeatureNamespaceSelector == nil {
		return nil
	}
	labelMap, err := metav1.LabelSelectorAsSelector(opts.NodeFeatureNamespaceSelector)
	if err != nil {
		klog.ErrorS(err, "failed to convert label selector to map", "selector", opts.NodeFeatureNamespaceSelector)
		return err
	}
	c.namespaceLister, err = newNamespaceLister(opts.K8sClient, labelMap)
	if err != nil {
		klog.ErrorS(err, "coudn't create namespace lister")
		return err
	}
	return nil
}

// addNodeFeatureInformer registers the informer and event handlers for
// NodeFeature objects.
func (c *nfdController) addNodeFeatureInformer(informerFactory nfdinformers.SharedInformerFactory, listSize int64) error {
	tweakListOpts := func(opts *metav1.ListOptions) {
		// Tweak list opts on initial sync to avoid timeouts on the apiserver.
		// NodeFeature objects are huge and the Kubernetes apiserver
		// (v1.30) experiences http handler timeouts when the resource
		// version is set to some non-empty value
		// https://github.com/kubernetes/kubernetes/blob/ace55542575fb098b3e413692bbe2bc20d2348ba/staging/src/k8s.io/apiserver/pkg/storage/cacher/cacher.go#L600-L616 if you set resource version to 0
		// it serves the request from apiservers cache and doesn't use pagination otherwise pagination will default to 500
		// so that's why this is required on large clusters
		// So by setting this we're making it go to ETCD instead of from api-server cache, there's some WIP in k/k
		// that seems to imply they're working on improving this behavior where you'll be able to paginate from apiserver cache
		// it's not supported yet (2/2025), would be good to track this though kubernetes/kubernetes#108003
		if opts.ResourceVersion == "0" {
			opts.ResourceVersion = ""
		}
		opts.Limit = listSize // value of 0 disables pagination
	}

	featureInformer := nfdinformersv1alpha1.New(informerFactory, "", tweakListOpts).NodeFeatures()
	if _, err := featureInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onNodeFeatureAdd,
		UpdateFunc: c.onNodeFeatureUpdate,
		DeleteFunc: c.onNodeFeatureDelete,
	}); err != nil {
		return err
	}
	c.featureLister = featureInformer.Lister()
	return nil
}

func (c *nfdController) onNodeFeatureAdd(obj interface{}) {
	nfr := obj.(*nfdv1alpha1.NodeFeature)
	klog.V(2).InfoS("NodeFeature added", "nodefeature", klog.KObj(nfr))
	if c.isNamespaceSelected(nfr.Namespace) {
		c.updateOneNode("NodeFeature", nfr)
	} else {
		klog.V(2).InfoS("NodeFeature namespace is not selected, skipping", "nodefeature", klog.KObj(nfr))
	}
	if !c.disableNodeFeatureGroup {
		c.updateAllNodeFeatureGroups()
	}
}

func (c *nfdController) onNodeFeatureUpdate(oldObj, newObj interface{}) {
	if !specChanged(oldObj, newObj, func(o *nfdv1alpha1.NodeFeature) any { return o.Spec }) {
		return
	}
	nfr := newObj.(*nfdv1alpha1.NodeFeature)
	klog.V(2).InfoS("NodeFeature updated", "nodefeature", klog.KObj(nfr))
	c.updateOneNode("NodeFeature", nfr)
	if !c.disableNodeFeatureGroup {
		c.updateAllNodeFeatureGroups()
	}
}

func (c *nfdController) onNodeFeatureDelete(obj interface{}) {
	// obj may be a DeletedFinalStateUnknown if the watch was missed
	if deletedFinalStateUnknown, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		klog.V(2).InfoS("found stale NodeFeature object", "object", obj)
		obj = deletedFinalStateUnknown.Obj
	}

	nfr, ok := obj.(*nfdv1alpha1.NodeFeature)
	if !ok {
		klog.ErrorS(fmt.Errorf("unexpected object type %T", obj), "cannot convert object to NodeFeature")
		return
	}

	klog.V(2).InfoS("NodeFeature deleted", "nodefeature", klog.KObj(nfr))
	// Mark the node as having a pending delete before queuing update.
	// This allows nfdAPIUpdateOneNode to distinguish between
	// "NodeFeature was deleted" vs "cache is incomplete".
	if c.onDelete != nil {
		nodeName, err := getNodeNameForObj(nfr)
		if err != nil {
			// Fallback: use the NodeFeature's name as the node name.
			// By convention, nfd-worker creates NodeFeatures with name == nodeName.
			// This may mark a wrong node for pending delete (for third-party
			// NodeFeatures), but that's harmless—it just gets consumed without effect.
			klog.V(2).InfoS("failed to get node name from label, falling back to object name",
				"nodefeature", klog.KObj(nfr), "error", err)
			nodeName = nfr.Name
		}
		c.onDelete(nodeName)
	}
	c.updateOneNode("NodeFeature", nfr)
	if !c.disableNodeFeatureGroup {
		c.updateAllNodeFeatureGroups()
	}
}

// addNodeFeatureRuleInformer registers the informer and event handlers for
// NodeFeatureRule objects.
func (c *nfdController) addNodeFeatureRuleInformer(informerFactory nfdinformers.SharedInformerFactory) error {
	nodeFeatureRuleInformer := informerFactory.Nfd().V1alpha1().NodeFeatureRules()
	if _, err := nodeFeatureRuleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			klog.V(2).InfoS("NodeFeatureRule added", "nodefeaturerule", klog.KObj(object.(metav1.Object)))
			c.updateAllNodes()
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			if !specChanged(oldObject, newObject, func(o *nfdv1alpha1.NodeFeatureRule) any { return o.Spec }) {
				return
			}
			klog.V(2).InfoS("NodeFeatureRule updated", "nodefeaturerule", klog.KObj(newObject.(metav1.Object)))
			c.updateAllNodes()
		},
		DeleteFunc: func(object interface{}) {
			klog.V(2).InfoS("NodeFeatureRule deleted", "nodefeaturerule", klog.KObj(object.(metav1.Object)))
			c.updateAllNodes()
		},
	}); err != nil {
		return err
	}
	c.ruleLister = nodeFeatureRuleInformer.Lister()
	return nil
}

// addNodeFeatureGroupInformer registers the informer and event handlers for
// NodeFeatureGroup objects.
func (c *nfdController) addNodeFeatureGroupInformer(informerFactory nfdinformers.SharedInformerFactory) error {
	nodeFeatureGroupInformer := informerFactory.Nfd().V1alpha1().NodeFeatureGroups()
	if _, err := nodeFeatureGroupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onNodeFeatureGroupAdd,
		UpdateFunc: c.onNodeFeatureGroupUpdate,
		DeleteFunc: c.onNodeFeatureGroupDelete,
	}); err != nil {
		return err
	}
	c.featureGroupLister = nodeFeatureGroupInformer.Lister()
	return nil
}

func (c *nfdController) onNodeFeatureGroupAdd(obj interface{}) {
	nfg := obj.(*nfdv1alpha1.NodeFeatureGroup)
	klog.V(2).InfoS("NodeFeatureGroup added", "nodeFeatureGroup", klog.KObj(nfg))
	c.updateNodeFeatureGroup(nfg.Name)
}

func (c *nfdController) onNodeFeatureGroupUpdate(oldObj, newObj interface{}) {
	if !specChanged(oldObj, newObj, func(o *nfdv1alpha1.NodeFeatureGroup) any { return o.Spec }) {
		return
	}
	nfg := newObj.(*nfdv1alpha1.NodeFeatureGroup)
	klog.V(2).InfoS("NodeFeatureGroup updated", "nodeFeatureGroup", klog.KObj(nfg))
	c.updateNodeFeatureGroup(nfg.Name)
}

func (c *nfdController) onNodeFeatureGroupDelete(obj interface{}) {
	// obj may be a DeletedFinalStateUnknown if the watch was missed
	if deletedFinalStateUnknown, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		klog.V(2).InfoS("found stale NodeFeatureGroup object", "object", obj)
		obj = deletedFinalStateUnknown.Obj
	}

	nfg, ok := obj.(*nfdv1alpha1.NodeFeatureGroup)
	if !ok {
		klog.ErrorS(fmt.Errorf("unexpected object type %T", obj), "cannot convert object to NodeFeatureGroup")
		return
	}

	klog.V(2).InfoS("NodeFeatureGroup deleted", "nodeFeatureGroup", klog.KObj(nfg))
	c.updateNodeFeatureGroup(nfg.Name)
}

func (c *nfdController) stop() {
	close(c.stopChan)
	// namespaceLister is only created when a NodeFeature namespace selector is
	// configured, so it may be nil (the default).
	if c.namespaceLister != nil {
		c.namespaceLister.stop()
	}
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

func (c *nfdController) updateOneNode(typ string, obj metav1.Object) {
	nodeName, err := getNodeNameForObj(obj)
	if err != nil {
		klog.ErrorS(err, "failed to determine node name for object", "type", typ, "object", klog.KObj(obj))
		return
	}
	select {
	case c.updateOneNodeChan <- nodeName:
	case <-c.stopChan:
	}
}

func (c *nfdController) isNamespaceSelected(namespace string) bool {
	// this means that the user didn't specify any namespace selector
	// which means that we allow all namespaces
	if c.namespaceLister == nil {
		return true
	}

	namespaces, err := c.namespaceLister.list()
	if err != nil {
		klog.ErrorS(err, "failed to query namespaces by the namespace lister")
		return false
	}

	for _, ns := range namespaces {
		if ns.Name == namespace {
			return true
		}
	}

	return false
}

func (c *nfdController) updateAllNodes() {
	select {
	case c.updateAllNodesChan <- struct{}{}:
	default:
	}
}

func (c *nfdController) updateNodeFeatureGroup(nodeFeatureGroup string) {
	select {
	case c.updateNodeFeatureGroupChan <- nodeFeatureGroup:
	case <-c.stopChan:
	}
}

func (c *nfdController) updateAllNodeFeatureGroups() {
	select {
	case c.updateAllNodeFeatureGroupsChan <- struct{}{}:
	default:
	}
}
