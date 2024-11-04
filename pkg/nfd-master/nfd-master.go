/*
Copyright 2019-2021 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	controller "k8s.io/kubernetes/pkg/controller"
	taintutils "k8s.io/kubernetes/pkg/util/taints"
	"sigs.k8s.io/yaml"

	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/nodefeaturerule"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/validate"
	nfdfeatures "sigs.k8s.io/node-feature-discovery/pkg/features"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	klogutils "sigs.k8s.io/node-feature-discovery/pkg/utils/klog"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// ExtendedResources are k8s extended resources which are created from discovered features.
type ExtendedResources map[string]string

// Annotations are used for NFD-related node metadata
type Annotations map[string]string

// Restrictions contains the restrictions on the NF and NFR Crs
type Restrictions struct {
	NodeFeatureNamespaceSelector *metav1.LabelSelector
	DisableLabels                bool
	DisableExtendedResources     bool
	DisableAnnotations           bool
	DenyNodeFeatureLabels        bool
	AllowOverwrite               bool
}

// NFDConfig contains the configuration settings of NfdMaster.
type NFDConfig struct {
	AutoDefaultNs     bool
	DenyLabelNs       utils.StringSetVal
	ExtraLabelNs      utils.StringSetVal
	LabelWhiteList    *regexp.Regexp
	NoPublish         bool
	EnableTaints      bool
	ResyncPeriod      utils.DurationVal
	LeaderElection    LeaderElectionConfig
	NfdApiParallelism int
	Klog              klogutils.KlogConfigOpts
	Restrictions      Restrictions
}

// LeaderElectionConfig contains the configuration for leader election
type LeaderElectionConfig struct {
	LeaseDuration utils.DurationVal
	RenewDeadline utils.DurationVal
	RetryPeriod   utils.DurationVal
}

// ConfigOverrideArgs are args that override config file options
type ConfigOverrideArgs struct {
	DenyLabelNs       *utils.StringSetVal
	ExtraLabelNs      *utils.StringSetVal
	LabelWhiteList    *utils.RegexpVal
	EnableTaints      *bool
	NoPublish         *bool
	ResyncPeriod      *utils.DurationVal
	NfdApiParallelism *int
}

// Args holds command line arguments
type Args struct {
	ConfigFile           string
	Instance             string
	Klog                 map[string]*utils.KlogFlagVal
	Kubeconfig           string
	Port                 int
	Prune                bool
	Options              string
	EnableLeaderElection bool

	Overrides ConfigOverrideArgs
}

type deniedNs struct {
	normal   utils.StringSetVal
	wildcard utils.StringSetVal
}

type NfdMaster interface {
	Run() error
	Stop()
}

type nfdMaster struct {
	*nfdController

	args           Args
	namespace      string
	nodeName       string
	configFilePath string
	stop           chan struct{}
	kubeconfig     *restclient.Config
	k8sClient      k8sclient.Interface
	nfdClient      nfdclientset.Interface
	updaterPool    *updaterPool
	deniedNs
	config *NFDConfig
}

// NewNfdMaster creates a new NfdMaster server instance.
func NewNfdMaster(opts ...NfdMasterOption) (NfdMaster, error) {
	nfd := &nfdMaster{
		nodeName:  utils.NodeName(),
		namespace: utils.GetKubernetesNamespace(),
		stop:      make(chan struct{}),
	}

	for _, o := range opts {
		o.apply(nfd)
	}

	if nfd.args.Instance != "" {
		if ok, _ := regexp.MatchString(`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`, nfd.args.Instance); !ok {
			return nfd, fmt.Errorf("invalid -instance %q: instance name "+
				"must start and end with an alphanumeric character and may only contain "+
				"alphanumerics, `-`, `_` or `.`", nfd.args.Instance)
		}
	}

	if nfd.args.ConfigFile != "" {
		nfd.configFilePath = filepath.Clean(nfd.args.ConfigFile)
	}

	// k8sClient might've been set via opts by tests
	if nfd.k8sClient == nil {
		kubeconfig, err := utils.GetKubeconfig(nfd.args.Kubeconfig)
		if err != nil {
			return nfd, err
		}
		nfd.kubeconfig = kubeconfig
		cli, err := k8sclient.NewForConfig(kubeconfig)
		if err != nil {
			return nfd, err
		}
		nfd.k8sClient = cli
	}

	// nfdClient
	if nfd.kubeconfig != nil {
		kubeconfig, err := utils.GetKubeconfig(nfd.args.Kubeconfig)
		if err != nil {
			return nfd, err
		}
		nfd.kubeconfig = kubeconfig
		c, err := nfdclientset.NewForConfig(nfd.kubeconfig)
		if err != nil {
			return nfd, err
		}
		nfd.nfdClient = c
	}

	nfd.updaterPool = newUpdaterPool(nfd)

	return nfd, nil
}

// NfdMasterOption sets properties of the NfdMaster instance.
type NfdMasterOption interface {
	apply(*nfdMaster)
}

// WithArgs is used for passing settings from command line arguments.
func WithArgs(args *Args) NfdMasterOption {
	return &nfdMasterOpt{f: func(n *nfdMaster) { n.args = *args }}
}

// WithKuberneteClient forces to use the given kubernetes client, without
// initializing one from kubeconfig.
func WithKubernetesClient(cli k8sclient.Interface) NfdMasterOption {
	return &nfdMasterOpt{f: func(n *nfdMaster) { n.k8sClient = cli }}
}

type nfdMasterOpt struct {
	f func(*nfdMaster)
}

func (f *nfdMasterOpt) apply(n *nfdMaster) {
	f.f(n)
}

func newDefaultConfig() *NFDConfig {
	return &NFDConfig{
		DenyLabelNs:       utils.StringSetVal{},
		ExtraLabelNs:      utils.StringSetVal{},
		NoPublish:         false,
		AutoDefaultNs:     true,
		NfdApiParallelism: 10,
		EnableTaints:      false,
		ResyncPeriod:      utils.DurationVal{Duration: time.Duration(1) * time.Hour},
		LeaderElection: LeaderElectionConfig{
			LeaseDuration: utils.DurationVal{Duration: time.Duration(15) * time.Second},
			RetryPeriod:   utils.DurationVal{Duration: time.Duration(2) * time.Second},
			RenewDeadline: utils.DurationVal{Duration: time.Duration(10) * time.Second},
		},
		Klog: make(map[string]string),
		Restrictions: Restrictions{
			DisableLabels:            false,
			DisableExtendedResources: false,
			DisableAnnotations:       false,
			AllowOverwrite:           true,
			DenyNodeFeatureLabels:    false,
		},
	}
}

// Run NfdMaster server. The method returns in case of fatal errors or if Stop()
// is called.
func (m *nfdMaster) Run() error {
	klog.InfoS("Node Feature Discovery Master", "version", version.Get(), "nodeName", m.nodeName, "namespace", m.namespace)
	if m.args.Instance != "" {
		klog.InfoS("Master instance", "instance", m.args.Instance)
	}

	// Read configuration
	if err := m.configure(m.configFilePath, m.args.Options); err != nil {
		return err
	}

	if m.args.Prune {
		return m.prune()
	}

	if err := m.startNfdApiController(); err != nil {
		return err
	}

	m.updaterPool.start(m.config.NfdApiParallelism)

	if !m.config.NoPublish {
		err := m.updateMasterNode()
		if err != nil {
			return fmt.Errorf("failed to update master node: %w", err)
		}
	}

	httpMux := http.NewServeMux()

	// Register to metrics server
	promRegistry := prometheus.NewRegistry()
	promRegistry.MustRegister(
		buildInfo,
		nodeUpdateRequests,
		nodeUpdates,
		nodeUpdateFailures,
		nodeLabelsRejected,
		nodeERsRejected,
		nodeTaintsRejected,
		nfrProcessingTime,
		nfrProcessingErrors)
	httpMux.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}))
	registerVersion(version.Get())

	// Run updater that handles events from the nfd CRD API.
	if m.nfdController != nil {
		if m.args.EnableLeaderElection {
			go m.nfdAPIUpdateHandlerWithLeaderElection()
		} else {
			go m.nfdAPIUpdateHandler()
		}
	}

	// Register health probe (at this point we're "ready and live")
	httpMux.HandleFunc("/healthz", m.Healthz)

	// Start HTTP server
	httpServer := http.Server{Addr: fmt.Sprintf(":%d", m.args.Port), Handler: httpMux}
	go func() {
		klog.InfoS("http server starting", "port", httpServer.Addr)
		klog.InfoS("http server stopped", "exitCode", httpServer.ListenAndServe())
	}()
	defer httpServer.Close()

	<-m.stop
	klog.InfoS("shutting down nfd-master")
	return nil
}

func (m *nfdMaster) Healthz(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

// nfdAPIUpdateHandler handles events from the nfd API controller.
func (m *nfdMaster) nfdAPIUpdateHandler() {
	// We want to unconditionally update all nodes at startup
	updateAll := true
	updateNodes := make(map[string]struct{})
	nodeFeatureGroup := make(map[string]struct{})
	updateAllNodeFeatureGroups := false
	rateLimit := time.After(time.Second)
	for {
		select {
		case <-m.nfdController.updateAllNodesChan:
			updateAll = true
		case nodeName := <-m.nfdController.updateOneNodeChan:
			updateNodes[nodeName] = struct{}{}
		case <-m.nfdController.updateAllNodeFeatureGroupsChan:
			updateAllNodeFeatureGroups = true
		case nodeFeatureGroupName := <-m.nfdController.updateNodeFeatureGroupChan:
			nodeFeatureGroup[nodeFeatureGroupName] = struct{}{}
		case <-rateLimit:
			// NodeFeature
			errUpdateAll := false
			if updateAll {
				if err := m.nfdAPIUpdateAllNodes(); err != nil {
					klog.ErrorS(err, "failed to update nodes")
					errUpdateAll = true
				}
			} else {
				for nodeName := range updateNodes {
					m.updaterPool.addNode(nodeName)
				}
			}
			// NodeFeatureGroup
			errUpdateAllNFG := false
			if updateAllNodeFeatureGroups {
				if err := m.nfdAPIUpdateAllNodeFeatureGroups(); err != nil {
					klog.ErrorS(err, "failed to update NodeFeatureGroups")
					errUpdateAllNFG = true
				}
			} else {
				for nodeFeatureGroupName := range nodeFeatureGroup {
					m.updaterPool.addNodeFeatureGroup(nodeFeatureGroupName)
				}
			}

			// Reset "work queue" and timer
			updateAll = errUpdateAll
			updateAllNodeFeatureGroups = errUpdateAllNFG
			nodeFeatureGroup = map[string]struct{}{}
			updateNodes = map[string]struct{}{}
			rateLimit = time.After(time.Second)
		}
	}
}

// Stop NfdMaster
func (m *nfdMaster) Stop() {
	if m.nfdController != nil {
		m.nfdController.stop()
	}

	m.updaterPool.stop()

	close(m.stop)
}

// Prune erases all NFD related properties from the node objects of the cluster.
func (m *nfdMaster) prune() error {
	if m.config.NoPublish {
		klog.InfoS("skipping pruning of nodes as noPublish config option is set")
		return nil
	}

	nodes, err := getNodes(m.k8sClient)
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		klog.InfoS("pruning node...", "nodeName", node.Name)

		// Prune labels and extended resources
		err := m.updateNodeObject(m.k8sClient, &node, Labels{}, Annotations{}, ExtendedResources{}, []corev1.Taint{})
		if err != nil {
			nodeUpdateFailures.Inc()
			return fmt.Errorf("failed to prune node %q: %v", node.Name, err)
		}

		// Prune annotations
		node, err := getNode(m.k8sClient, node.Name)
		if err != nil {
			return err
		}
		maps.DeleteFunc(node.Annotations, func(k, v string) bool {
			return strings.HasPrefix(k, m.instanceAnnotation(nfdv1alpha1.AnnotationNs))
		})
		_, err = m.k8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to prune annotations from node %q: %v", node.Name, err)
		}
	}
	return nil
}

// Update annotations on the node where nfd-master is running. Currently the
// only function is to remove the deprecated
// "nfd.node.kubernetes.io/master.version" annotation, if it exists.
// TODO: Drop when nfdv1alpha1.MasterVersionAnnotation is removed.
func (m *nfdMaster) updateMasterNode() error {
	node, err := getNode(m.k8sClient, m.nodeName)
	if err != nil {
		return err
	}

	// Advertise NFD version as an annotation
	p := createPatches(sets.New([]string{m.instanceAnnotation(nfdv1alpha1.MasterVersionAnnotation)}...),
		node.Annotations,
		nil,
		"/metadata/annotations", m.config.Restrictions.AllowOverwrite)

	err = patchNode(m.k8sClient, node.Name, p)
	if err != nil {
		return fmt.Errorf("failed to patch node annotations: %w", err)
	}

	return nil
}

// Filter labels by namespace and name whitelist, and, turn selected labels
// into extended resources. This function also handles proper namespacing of
// labels and ERs, i.e. adds the possibly missing default namespace for labels.
func (m *nfdMaster) filterFeatureLabels(labels Labels, features *nfdv1alpha1.Features) Labels {
	outLabels := Labels{}
	for name, value := range labels {
		if value, err := m.filterFeatureLabel(name, value, features); err != nil {
			klog.ErrorS(err, "ignoring label", "labelKey", name, "labelValue", value)
			nodeLabelsRejected.Inc()
		} else {
			outLabels[name] = value
		}
	}

	if len(outLabels) > 0 && m.config.Restrictions.DisableLabels {
		klog.V(2).InfoS("node labels are disabled in configuration (restrictions.disableLabels=true)")
		outLabels = Labels{}
	}

	return outLabels
}

func (m *nfdMaster) filterFeatureLabel(name, value string, features *nfdv1alpha1.Features) (string, error) {
	// Check if Value is dynamic
	var filteredValue string
	if strings.HasPrefix(value, "@") {
		dynamicValue, err := getDynamicValue(value, features)
		if err != nil {
			return "", err
		}
		filteredValue = dynamicValue
	} else {
		filteredValue = value
	}

	// Validate
	ns, base := splitNs(name)
	err := validate.Label(name, filteredValue)
	if err == validate.ErrNSNotAllowed || isNamespaceDenied(ns, m.deniedNs.wildcard, m.deniedNs.normal) {
		if _, ok := m.config.ExtraLabelNs[ns]; !ok {
			return "", fmt.Errorf("namespace %q is not allowed", ns)
		}
	} else if err != nil {
		return "", err
	}

	// Skip if label doesn't match labelWhiteList
	if m.config.LabelWhiteList != nil && !m.config.LabelWhiteList.MatchString(base) {
		return "", fmt.Errorf("%s (%s) does not match the whitelist (%s)", base, name, m.config.LabelWhiteList.String())
	}

	return filteredValue, nil
}

func getDynamicValue(value string, features *nfdv1alpha1.Features) (string, error) {
	// value is a string in the form of attribute.featureset.elements
	split := strings.SplitN(value[1:], ".", 3)
	if len(split) != 3 {
		return "", fmt.Errorf("value %s is not in the form of '@domain.feature.element'", value)
	}
	featureName := split[0] + "." + split[1]
	elementName := split[2]
	attrFeatureSet, ok := features.Attributes[featureName]
	if !ok {
		return "", fmt.Errorf("feature %s not found", featureName)
	}
	element, ok := attrFeatureSet.Elements[elementName]
	if !ok {
		return "", fmt.Errorf("element %s not found on feature %s", elementName, featureName)
	}
	return element, nil
}

func filterTaints(taints []corev1.Taint) []corev1.Taint {
	outTaints := []corev1.Taint{}

	for _, taint := range taints {
		if err := validate.Taint(&taint); err != nil {
			klog.ErrorS(err, "ignoring taint", "taint", taint)
			nodeTaintsRejected.Inc()
		} else {
			outTaints = append(outTaints, taint)
		}
	}

	return outTaints
}

func isNamespaceDenied(labelNs string, wildcardDeniedNs map[string]struct{}, normalDeniedNs map[string]struct{}) bool {
	for deniedNs := range normalDeniedNs {
		if labelNs == deniedNs {
			return true
		}
	}
	for deniedNs := range wildcardDeniedNs {
		if strings.HasSuffix(labelNs, deniedNs) {
			return true
		}
	}
	return false
}

func (m *nfdMaster) nfdAPIUpdateAllNodes() error {
	klog.InfoS("will process all nodes in the cluster")

	nodes, err := getNodes(m.k8sClient)
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		m.updaterPool.addNode(node.Name)
	}

	return nil
}

// getAndMergeNodeFeatures merges the NodeFeature objects of the given node into a single NodeFeatureSpec.
// The Name field of the returned NodeFeatureSpec contains the node name.
func (m *nfdMaster) getAndMergeNodeFeatures(nodeName string) (*nfdv1alpha1.NodeFeature, error) {
	nodeFeatures := &nfdv1alpha1.NodeFeature{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}

	sel := k8sLabels.SelectorFromSet(k8sLabels.Set{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodeName})
	objs, err := m.nfdController.featureLister.List(sel)
	if err != nil {
		return &nfdv1alpha1.NodeFeature{}, fmt.Errorf("failed to get NodeFeature resources for node %q: %w", nodeName, err)
	}

	filteredObjs := []*nfdv1alpha1.NodeFeature{}
	for _, obj := range objs {
		if m.isNamespaceSelected(obj.Namespace) {
			filteredObjs = append(filteredObjs, obj)
		}
	}

	// Node without a running NFD-Worker
	if len(filteredObjs) == 0 {
		return &nfdv1alpha1.NodeFeature{}, nil
	}

	// Sort our objects
	sort.Slice(filteredObjs, func(i, j int) bool {
		// Objects in our nfd namespace gets into the beginning of the list
		if filteredObjs[i].Namespace == m.namespace && filteredObjs[j].Namespace != m.namespace {
			return true
		}
		if filteredObjs[i].Namespace != m.namespace && filteredObjs[j].Namespace == m.namespace {
			return false
		}
		// After the nfd namespace, sort objects by their name
		if filteredObjs[i].Name != filteredObjs[j].Name {
			return filteredObjs[i].Name < filteredObjs[j].Name
		}
		// Objects with the same name are sorted by their namespace
		return filteredObjs[i].Namespace < filteredObjs[j].Namespace
	})

	if len(filteredObjs) > 0 {
		// Merge in features
		//
		// NOTE: changing the rule api to support handle multiple objects instead
		// of merging would probably perform better with lot less data to copy.
		features := filteredObjs[0].Spec.DeepCopy()

		if m.config.Restrictions.DenyNodeFeatureLabels && m.isThirdPartyNodeFeature(*filteredObjs[0], nodeName, m.namespace) {
			klog.V(2).InfoS("node feature labels are disabled in configuration (restrictions.denyNodeFeatureLabels=true)")
			features.Labels = nil
		}

		if !nfdfeatures.NFDFeatureGate.Enabled(nfdfeatures.DisableAutoPrefix) && m.config.AutoDefaultNs {
			features.Labels = addNsToMapKeys(features.Labels, nfdv1alpha1.FeatureLabelNs)
		}

		for _, o := range filteredObjs[1:] {
			s := o.Spec.DeepCopy()
			if m.config.Restrictions.DenyNodeFeatureLabels && m.isThirdPartyNodeFeature(*o, nodeName, m.namespace) {
				klog.V(2).InfoS("node feature labels are disabled in configuration (restrictions.denyNodeFeatureLabels=true)")
				s.Labels = nil
			}

			if !nfdfeatures.NFDFeatureGate.Enabled(nfdfeatures.DisableAutoPrefix) && m.config.AutoDefaultNs {
				s.Labels = addNsToMapKeys(s.Labels, nfdv1alpha1.FeatureLabelNs)
			}

			s.MergeInto(features)
		}

		// Set the merged features to the NodeFeature object
		nodeFeatures.Spec = *features

		klog.V(4).InfoS("merged nodeFeatureSpecs", "newNodeFeatureSpec", utils.DelayedDumper(features))
	}

	return nodeFeatures, nil
}

// isThirdPartyNodeFeature determines whether a node feature is a third party one or created by nfd-worker
func (m *nfdMaster) isThirdPartyNodeFeature(nodeFeature nfdv1alpha1.NodeFeature, nodeName, namespace string) bool {
	return nodeFeature.Namespace != namespace || nodeFeature.Name != nodeName
}

func (m *nfdMaster) nfdAPIUpdateOneNode(cli k8sclient.Interface, node *corev1.Node) error {
	if m.nfdController == nil || m.nfdController.featureLister == nil {
		return nil
	}

	// Merge all NodeFeature objects into a single NodeFeatureSpec
	nodeFeatures, err := m.getAndMergeNodeFeatures(node.Name)
	if err != nil {
		return fmt.Errorf("failed to merge NodeFeature objects for node %q: %w", node.Name, err)
	}

	// Update node labels et al. This may also mean removing all NFD-owned
	// labels (et al.), for example  in the case no NodeFeature objects are
	// present.
	if err := m.refreshNodeFeatures(cli, node, nodeFeatures.Spec.Labels, &nodeFeatures.Spec.Features); err != nil {
		return err
	}

	return nil
}

func (m *nfdMaster) nfdAPIUpdateAllNodeFeatureGroups() error {
	klog.V(1).InfoS("updating all NodeFeatureGroups")

	nodeFeatureGroupsList, err := m.nfdController.featureGroupLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to get NodeFeatureGroup objects: %w", err)
	}

	if len(nodeFeatureGroupsList) > 0 {
		for _, nodeFeatureGroup := range nodeFeatureGroupsList {
			m.updaterPool.nfgQueue.Add(nodeFeatureGroup.Name)
		}
	} else {
		klog.V(2).InfoS("no NodeFeatureGroup objects found")
	}

	return nil
}

func (m *nfdMaster) nfdAPIUpdateNodeFeatureGroup(nfdClient nfdclientset.Interface, nodeFeatureGroup *nfdv1alpha1.NodeFeatureGroup) error {
	klog.V(2).InfoS("evaluating NodeFeatureGroup", "nodeFeatureGroup", klog.KObj(nodeFeatureGroup))
	if m.nfdController == nil || m.nfdController.featureLister == nil {
		return nil
	}

	// Get all Nodes
	nodes, err := getNodes(m.k8sClient)
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}
	nodeFeaturesList := make([]*nfdv1alpha1.NodeFeature, 0)
	for _, node := range nodes.Items {
		// Merge all NodeFeature objects into a single NodeFeatureSpec
		nodeFeatures, err := m.getAndMergeNodeFeatures(node.Name)
		if err != nil {
			return fmt.Errorf("failed to merge NodeFeature objects for node %q: %w", node.Name, err)
		}
		if nodeFeatures.Name == "" {
			// Nothing to do for this node
			continue
		}
		nodeFeaturesList = append(nodeFeaturesList, nodeFeatures)
	}

	// Execute rules and create matching groups
	nodePool := make([]nfdv1alpha1.FeatureGroupNode, 0)
	nodeGroupValidator := make(map[string]bool)
	for _, rule := range nodeFeatureGroup.Spec.Rules {
		for _, feature := range nodeFeaturesList {
			match, err := nodefeaturerule.ExecuteGroupRule(&rule, &feature.Spec.Features, true)
			if err != nil {
				klog.ErrorS(err, "failed to evaluate rule", "ruleName", rule.Name)
				continue
			}

			if match {
				klog.ErrorS(err, "failed to evaluate rule", "ruleName", rule.Name, "nodeName", feature.Name)
				system := feature.Spec.Features.Attributes["system.name"]
				nodeName := system.Elements["nodename"]
				if _, ok := nodeGroupValidator[nodeName]; !ok {
					nodePool = append(nodePool, nfdv1alpha1.FeatureGroupNode{
						Name: nodeName,
					})
					nodeGroupValidator[nodeName] = true
				}
			}
		}
	}

	// Update the NodeFeatureGroup object with the updated featureGroupRules
	nodeFeatureGroupUpdated := nodeFeatureGroup.DeepCopy()
	nodeFeatureGroupUpdated.Status.Nodes = nodePool

	if !apiequality.Semantic.DeepEqual(nodeFeatureGroup, nodeFeatureGroupUpdated) {
		klog.InfoS("updating NodeFeatureGroup object", "nodeFeatureGroup", klog.KObj(nodeFeatureGroup))
		nodeFeatureGroupUpdated, err = nfdClient.NfdV1alpha1().NodeFeatureGroups(m.namespace).UpdateStatus(context.TODO(), nodeFeatureGroupUpdated, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update NodeFeatureGroup object: %w", err)
		}
		klog.V(4).InfoS("NodeFeatureGroup object updated", "nodeFeatureGroup", utils.DelayedDumper(nodeFeatureGroupUpdated))
	} else {
		klog.V(1).InfoS("no changes in NodeFeatureGroup, object is up to date", "nodeFeatureGroup", klog.KObj(nodeFeatureGroup))
	}

	return nil
}

// filterExtendedResources filters extended resources and returns a map
// of valid extended resources.
func (m *nfdMaster) filterExtendedResources(features *nfdv1alpha1.Features, extendedResources ExtendedResources) ExtendedResources {
	outExtendedResources := ExtendedResources{}
	for name, value := range extendedResources {
		capacity, err := filterExtendedResource(name, value, features)
		if err != nil {
			klog.ErrorS(err, "failed to create extended resources", "extendedResourceName", name, "extendedResourceValue", value)
			nodeERsRejected.Inc()
		} else {
			outExtendedResources[name] = capacity
		}
	}
	return outExtendedResources
}

func filterExtendedResource(name, value string, features *nfdv1alpha1.Features) (string, error) {
	// Dynamic Value
	var filteredValue string
	if strings.HasPrefix(value, "@") {
		dynamicValue, err := getDynamicValue(value, features)
		if err != nil {
			return "", err
		}
		filteredValue = dynamicValue
	} else {
		filteredValue = value
	}

	// Validate
	err := validate.ExtendedResource(name, filteredValue)
	if err != nil {
		return "", err
	}

	return filteredValue, nil
}

func (m *nfdMaster) refreshNodeFeatures(cli k8sclient.Interface, node *corev1.Node, labels map[string]string, features *nfdv1alpha1.Features) error {
	if !nfdfeatures.NFDFeatureGate.Enabled(nfdfeatures.DisableAutoPrefix) && m.config.AutoDefaultNs {
		labels = addNsToMapKeys(labels, nfdv1alpha1.FeatureLabelNs)
	} else if labels == nil {
		labels = make(map[string]string)
	}

	crLabels, crAnnotations, crExtendedResources, crTaints := m.processNodeFeatureRule(node.Name, features)

	// Labels
	maps.Copy(labels, crLabels)
	labels = m.filterFeatureLabels(labels, features)

	// Extended resources
	extendedResources := m.filterExtendedResources(features, crExtendedResources)

	if len(extendedResources) > 0 && m.config.Restrictions.DisableExtendedResources {
		klog.V(2).InfoS("extended resources are disabled in configuration (restrictions.disableExtendedResources=true)")
		extendedResources = map[string]string{}
	}

	// Annotations
	annotations := m.filterFeatureAnnotations(crAnnotations)

	// Taints
	var taints []corev1.Taint
	if m.config.EnableTaints {
		taints = filterTaints(crTaints)
	}

	if m.config.NoPublish {
		klog.V(1).InfoS("node update skipped, NoPublish=true", "nodeName", node.Name)
		return nil
	}

	err := m.updateNodeObject(cli, node, labels, annotations, extendedResources, taints)
	if err != nil {
		klog.ErrorS(err, "failed to update node", "nodeName", node.Name)
		return err
	}

	return nil
}

// setTaints sets node taints and annotations based on the taints passed via
// nodeFeatureRule custom resorce. If empty list of taints is passed, currently
// NFD owned taints and annotations are removed from the node.
func (m *nfdMaster) setTaints(cli k8sclient.Interface, taints []corev1.Taint, node *corev1.Node) error {
	// De-serialize the taints annotation into corev1.Taint type for comparision below.
	var err error
	oldTaints := []corev1.Taint{}
	if val, ok := node.Annotations[nfdv1alpha1.NodeTaintsAnnotation]; ok {
		sts := strings.Split(val, ",")
		oldTaints, _, err = taintutils.ParseTaints(sts)
		if err != nil {
			return err
		}
	}

	// Delete old nfd-managed taints that are not found in the set of new taints.
	taintsUpdated := false
	newNode := node.DeepCopy()
	for _, taintToRemove := range oldTaints {
		if taintutils.TaintExists(taints, &taintToRemove) {
			continue
		}

		newTaints, removed := taintutils.DeleteTaint(newNode.Spec.Taints, &taintToRemove)
		if !removed {
			klog.V(1).InfoS("taint already deleted from node", "taint", taintToRemove)
		}
		taintsUpdated = taintsUpdated || removed
		newNode.Spec.Taints = newTaints
	}

	// Add new taints found in the set of new taints.
	for _, taint := range taints {
		var updated bool
		newNode, updated, err = taintutils.AddOrUpdateTaint(newNode, &taint)
		if err != nil {
			return fmt.Errorf("failed to add %q taint on node %v", taint, node.Name)
		}
		taintsUpdated = taintsUpdated || updated
	}

	if taintsUpdated {
		if err := controller.PatchNodeTaints(context.TODO(), cli, node.Name, node, newNode); err != nil {
			return fmt.Errorf("failed to patch the node %v", node.Name)
		}
		klog.InfoS("updated node taints", "nodeName", node.Name)
	}

	// Update node annotation that holds the taints managed by us
	newAnnotations := map[string]string{}
	if len(taints) > 0 {
		// Serialize the new taints into string and update the annotation
		// with that string.
		taintStrs := make([]string, 0, len(taints))
		for _, taint := range taints {
			taintStrs = append(taintStrs, taint.ToString())
		}
		newAnnotations[nfdv1alpha1.NodeTaintsAnnotation] = strings.Join(taintStrs, ",")
	}

	patches := createPatches(sets.New([]string{nfdv1alpha1.NodeTaintsAnnotation}...),
		node.Annotations, newAnnotations,
		"/metadata/annotations",
		m.config.Restrictions.AllowOverwrite,
	)
	if len(patches) > 0 {
		if err := patchNode(cli, node.Name, patches); err != nil {
			return fmt.Errorf("error while patching node object: %w", err)
		}
		klog.V(1).InfoS("patched node annotations for taints", "nodeName", node.Name)
	}
	return nil
}

func (m *nfdMaster) processNodeFeatureRule(nodeName string, features *nfdv1alpha1.Features) (Labels, Annotations, ExtendedResources, []corev1.Taint) {
	if m.nfdController == nil {
		return nil, nil, nil, nil
	}

	extendedResources := ExtendedResources{}
	labels := make(map[string]string)
	annotations := make(map[string]string)
	var taints []corev1.Taint
	ruleSpecs, err := m.nfdController.ruleLister.List(k8sLabels.Everything())
	sort.Slice(ruleSpecs, func(i, j int) bool {
		return ruleSpecs[i].Name < ruleSpecs[j].Name
	})

	if err != nil {
		klog.ErrorS(err, "failed to list NodeFeatureRule resources")
		return nil, nil, nil, nil
	}

	// Process all rule CRs
	processStart := time.Now()
	for _, spec := range ruleSpecs {
		t := time.Now()
		switch {
		case klog.V(3).Enabled():
			klog.InfoS("executing NodeFeatureRule", "nodefeaturerule", klog.KObj(spec), "nodeName", nodeName, "nodeFeatureRuleSpec", utils.DelayedDumper(spec.Spec))
		case klog.V(1).Enabled():
			klog.InfoS("executing NodeFeatureRule", "nodefeaturerule", klog.KObj(spec), "nodeName", nodeName)
		}
		for _, rule := range spec.Spec.Rules {
			ruleOut, err := nodefeaturerule.Execute(&rule, features, true)
			if err != nil {
				klog.ErrorS(err, "failed to process rule", "ruleName", rule.Name, "nodefeaturerule", klog.KObj(spec), "nodeName", nodeName)
				nfrProcessingErrors.Inc()
				continue
			}
			taints = append(taints, ruleOut.Taints...)

			l := ruleOut.Labels
			e := ruleOut.ExtendedResources
			a := ruleOut.Annotations
			if !nfdfeatures.NFDFeatureGate.Enabled(nfdfeatures.DisableAutoPrefix) && m.config.AutoDefaultNs {
				l = addNsToMapKeys(ruleOut.Labels, nfdv1alpha1.FeatureLabelNs)
				e = addNsToMapKeys(ruleOut.ExtendedResources, nfdv1alpha1.ExtendedResourceNs)
				a = addNsToMapKeys(ruleOut.Annotations, nfdv1alpha1.FeatureAnnotationNs)
			}
			maps.Copy(labels, l)
			maps.Copy(extendedResources, e)
			maps.Copy(annotations, a)

			// Feed back rule output to features map for subsequent rules to match
			features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Labels)
			features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Vars)
		}
		nfrProcessingTime.WithLabelValues(spec.Name, nodeName).Observe(time.Since(t).Seconds())
	}
	processingTime := time.Since(processStart)
	klog.V(2).InfoS("processed NodeFeatureRule objects", "nodeName", nodeName, "objectCount", len(ruleSpecs), "duration", processingTime)

	return labels, annotations, extendedResources, taints
}

// updateNodeObject ensures the Kubernetes node object is up to date,
// creating new labels and extended resources where necessary and removing
// outdated ones. Also updates the corresponding annotations.
func (m *nfdMaster) updateNodeObject(cli k8sclient.Interface, node *corev1.Node, labels Labels, featureAnnotations Annotations, extendedResources ExtendedResources, taints []corev1.Taint) error {
	annotations := make(Annotations)

	// Store names of labels in an annotation
	if len(labels) > 0 {
		labelKeys := make([]string, 0, len(labels))
		for key := range labels {
			// Drop the ns part for labels in the default ns
			labelKeys = append(labelKeys, strings.TrimPrefix(key, nfdv1alpha1.FeatureLabelNs+"/"))
		}
		sort.Strings(labelKeys)
		annotations[m.instanceAnnotation(nfdv1alpha1.FeatureLabelsAnnotation)] = strings.Join(labelKeys, ",")
	}

	// Store names of extended resources in an annotation
	if len(extendedResources) > 0 {
		extendedResourceKeys := make([]string, 0, len(extendedResources))
		for key := range extendedResources {
			// Drop the ns part if in the default ns
			extendedResourceKeys = append(extendedResourceKeys, strings.TrimPrefix(key, nfdv1alpha1.FeatureLabelNs+"/"))
		}
		sort.Strings(extendedResourceKeys)
		annotations[m.instanceAnnotation(nfdv1alpha1.ExtendedResourceAnnotation)] = strings.Join(extendedResourceKeys, ",")
	}

	// Store feature annotations
	if len(featureAnnotations) > 0 {
		// Store names of feature annotations in an annotation
		annotationKeys := make([]string, 0, len(featureAnnotations))
		for key := range featureAnnotations {
			// Drop the ns part for annotations in the default ns
			annotationKeys = append(annotationKeys, strings.TrimPrefix(key, nfdv1alpha1.FeatureAnnotationNs+"/"))
		}
		sort.Strings(annotationKeys)
		annotations[m.instanceAnnotation(nfdv1alpha1.FeatureAnnotationsTrackingAnnotation)] = strings.Join(annotationKeys, ",")
		maps.Copy(annotations, featureAnnotations)
	}

	// Create JSON patches for changes in labels and annotations
	oldLabels := stringToNsNames(node.Annotations[m.instanceAnnotation(nfdv1alpha1.FeatureLabelsAnnotation)], nfdv1alpha1.FeatureLabelNs)
	oldAnnotations := stringToNsNames(node.Annotations[m.instanceAnnotation(nfdv1alpha1.FeatureAnnotationsTrackingAnnotation)], nfdv1alpha1.FeatureAnnotationNs)
	patches := createPatches(sets.New(oldLabels...), node.Labels, labels, "/metadata/labels", m.config.Restrictions.AllowOverwrite)
	oldAnnotations = append(oldAnnotations, []string{
		m.instanceAnnotation(nfdv1alpha1.FeatureLabelsAnnotation),
		m.instanceAnnotation(nfdv1alpha1.ExtendedResourceAnnotation),
		m.instanceAnnotation(nfdv1alpha1.FeatureAnnotationsTrackingAnnotation),
		// Clean up deprecated/stale nfd version annotations
		m.instanceAnnotation(nfdv1alpha1.MasterVersionAnnotation),
		m.instanceAnnotation(nfdv1alpha1.WorkerVersionAnnotation)}...)
	patches = append(patches, createPatches(sets.New(oldAnnotations...), node.Annotations, annotations, "/metadata/annotations", m.config.Restrictions.AllowOverwrite)...)

	// patch node status with extended resource changes
	statusPatches := m.createExtendedResourcePatches(node, extendedResources)
	err := patchNodeStatus(cli, node.Name, statusPatches)
	if err != nil {
		return fmt.Errorf("error while patching extended resources: %w", err)
	}

	// Patch the node object in the apiserver
	err = patchNode(cli, node.Name, patches)
	if err != nil {
		return fmt.Errorf("error while patching node object: %w", err)
	}

	if len(patches) > 0 || len(statusPatches) > 0 {
		nodeUpdates.Inc()
		klog.InfoS("node updated", "nodeName", node.Name)
	} else {
		klog.V(1).InfoS("no updates to node", "nodeName", node.Name)
	}

	// Set taints
	err = m.setTaints(cli, taints, node)
	if err != nil {
		return err
	}

	return err
}

// createPatches is a generic helper that returns json patch operations to perform
func createPatches(removeKeys sets.Set[string], oldItems map[string]string, newItems map[string]string, jsonPath string, overwrite bool) []utils.JsonPatch {
	patches := []utils.JsonPatch{}

	// Determine items to remove
	for key := range removeKeys {
		if _, ok := oldItems[key]; ok {
			if _, ok := newItems[key]; !ok {
				patches = append(patches, utils.NewJsonPatch("remove", jsonPath, key, ""))
			}
		}
	}

	// Determine items to add or replace
	for key, newVal := range newItems {
		if oldVal, ok := oldItems[key]; ok {
			if newVal != oldVal && (!removeKeys.Has(key) || overwrite) {
				patches = append(patches, utils.NewJsonPatch("replace", jsonPath, key, newVal))
			}
		} else {
			patches = append(patches, utils.NewJsonPatch("add", jsonPath, key, newVal))
		}
	}

	return patches
}

// createExtendedResourcePatches returns a slice of operations to perform on
// the node status
func (m *nfdMaster) createExtendedResourcePatches(n *corev1.Node, extendedResources ExtendedResources) []utils.JsonPatch {
	patches := []utils.JsonPatch{}

	// Form a list of namespaced resource names managed by us
	oldResources := stringToNsNames(n.Annotations[m.instanceAnnotation(nfdv1alpha1.ExtendedResourceAnnotation)], nfdv1alpha1.FeatureLabelNs)

	// figure out which resources to remove
	for _, resource := range oldResources {
		if _, ok := n.Status.Capacity[corev1.ResourceName(resource)]; ok {
			// check if the ext resource is still needed
			if _, extResNeeded := extendedResources[resource]; !extResNeeded {
				patches = append(patches, utils.NewJsonPatch("remove", "/status/capacity", resource, ""))
				patches = append(patches, utils.NewJsonPatch("remove", "/status/allocatable", resource, ""))
			}
		}
	}

	// figure out which resources to replace and which to add
	for resource, value := range extendedResources {
		// check if the extended resource already exists with the same capacity in the node
		if quantity, ok := n.Status.Capacity[corev1.ResourceName(resource)]; ok {
			val, _ := quantity.AsInt64()
			if strconv.FormatInt(val, 10) != value {
				patches = append(patches, utils.NewJsonPatch("replace", "/status/capacity", resource, value))
				patches = append(patches, utils.NewJsonPatch("replace", "/status/allocatable", resource, value))
			}
		} else {
			patches = append(patches, utils.NewJsonPatch("add", "/status/capacity", resource, value))
			// "allocatable" gets added implicitly after adding to capacity
		}
	}

	return patches
}

// Parse configuration options
func (m *nfdMaster) configure(filepath string, overrides string) error {
	// Create a new default config
	c := newDefaultConfig()

	// Try to read and parse config file
	if filepath != "" {
		data, err := os.ReadFile(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				klog.InfoS("config file not found, using defaults", "path", filepath)
			} else {
				return fmt.Errorf("error reading config file: %w", err)
			}
		} else {
			err = yaml.Unmarshal(data, c)
			if err != nil {
				return fmt.Errorf("failed to parse config file: %w", err)
			}

			klog.InfoS("configuration file parsed", "path", filepath)
		}
	}

	// Parse config overrides
	if err := yaml.Unmarshal([]byte(overrides), c); err != nil {
		return fmt.Errorf("failed to parse -options: %w", err)
	}
	if m.args.Overrides.NoPublish != nil {
		c.NoPublish = *m.args.Overrides.NoPublish
	}
	if m.args.Overrides.DenyLabelNs != nil {
		c.DenyLabelNs = *m.args.Overrides.DenyLabelNs
	}
	if m.args.Overrides.ExtraLabelNs != nil {
		c.ExtraLabelNs = *m.args.Overrides.ExtraLabelNs
	}
	if m.args.Overrides.EnableTaints != nil {
		c.EnableTaints = *m.args.Overrides.EnableTaints
	}
	if m.args.Overrides.LabelWhiteList != nil {
		c.LabelWhiteList = &m.args.Overrides.LabelWhiteList.Regexp
	}
	if m.args.Overrides.ResyncPeriod != nil {
		c.ResyncPeriod = *m.args.Overrides.ResyncPeriod
	}
	if m.args.Overrides.NfdApiParallelism != nil {
		c.NfdApiParallelism = *m.args.Overrides.NfdApiParallelism
	}

	if c.NfdApiParallelism <= 0 {
		return fmt.Errorf("the maximum number of concurrent labelers should be a non-zero positive number")
	}

	m.config = c

	if err := klogutils.MergeKlogConfiguration(m.args.Klog, c.Klog); err != nil {
		return err
	}

	// Pre-process DenyLabelNS into 2 lists: one for normal ns, and the other for wildcard ns
	normalDeniedNs, wildcardDeniedNs := preProcessDeniedNamespaces(c.DenyLabelNs)
	m.deniedNs.normal = normalDeniedNs
	m.deniedNs.wildcard = wildcardDeniedNs

	klog.InfoS("configuration successfully updated", "configuration", utils.DelayedDumper(m.config))

	return nil
}

// addNsToMapKeys creates a copy of a map with the namespace (prefix) added to
// unprefixed keys. Prefixed keys in the input map will take presedence, i.e.
// if the input contains both prefixed (say "prefix/name") and unprefixed
// ("name") name the unprefixed key will be ignored.
func addNsToMapKeys(in map[string]string, nsToAdd string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.Contains(k, "/") {
			out[k] = v
		} else {
			fqn := path.Join(nsToAdd, k)
			if _, ok := in[fqn]; !ok {
				out[fqn] = v
			}
		}
	}
	return out
}

// addNs adds a namespace if one isn't already found from src string
func addNs(src string, nsToAdd string) string {
	if strings.Contains(src, "/") {
		return src
	}
	return path.Join(nsToAdd, src)
}

// splitNs splits a name into its namespace and name parts
// Ported to Validate
func splitNs(fullname string) (string, string) {
	split := strings.SplitN(fullname, "/", 2)
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", fullname
}

// stringToNsNames is a helper for converting a string of comma-separated names
// into a slice of fully namespaced names
func stringToNsNames(cslist, ns string) []string {
	var names []string
	if cslist != "" {
		names = strings.Split(cslist, ",")
		for i, name := range names {
			// Expect that names may omit the ns part
			names[i] = addNs(name, ns)
		}
	}
	return names
}

// Separate denied namespaces into two lists:
// one contains wildcard namespaces the other contains normal namespaces
func preProcessDeniedNamespaces(deniedNs map[string]struct{}) (normalDeniedNs map[string]struct{}, wildcardDeniedNs map[string]struct{}) {
	normalDeniedNs = map[string]struct{}{}
	wildcardDeniedNs = map[string]struct{}{}
	for ns := range deniedNs {
		if strings.HasPrefix(ns, "*") {
			trimedNs := strings.TrimLeft(ns, "*")
			wildcardDeniedNs[trimedNs] = struct{}{}
		} else {
			normalDeniedNs[ns] = struct{}{}
		}
	}
	return
}

func (m *nfdMaster) instanceAnnotation(name string) string {
	if m.args.Instance == "" {
		return name
	}
	return m.args.Instance + "." + name
}

func (m *nfdMaster) startNfdApiController() error {
	kubeconfig, err := utils.GetKubeconfig(m.args.Kubeconfig)
	if err != nil {
		return err
	}
	klog.InfoS("starting the nfd api controller")
	m.nfdController, err = newNfdController(kubeconfig, nfdApiControllerOptions{
		ResyncPeriod:                 m.config.ResyncPeriod.Duration,
		K8sClient:                    m.k8sClient,
		NodeFeatureNamespaceSelector: m.config.Restrictions.NodeFeatureNamespaceSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize CRD controller: %w", err)
	}
	return nil
}

func (m *nfdMaster) nfdAPIUpdateHandlerWithLeaderElection() {
	ctx := context.Background()
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "nfd-master.nfd.kubernetes.io",
			Namespace: m.namespace,
		},
		Client: m.k8sClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			// add uuid to prevent situation where 2 nfd-master nodes run on same node
			Identity: m.nodeName + "_" + uuid.NewString(),
		},
	}
	config := leaderelection.LeaderElectionConfig{
		Lock: lock,
		// make it configurable?
		LeaseDuration: m.config.LeaderElection.LeaseDuration.Duration,
		RetryPeriod:   m.config.LeaderElection.RetryPeriod.Duration,
		RenewDeadline: m.config.LeaderElection.RenewDeadline.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				m.nfdAPIUpdateHandler()
			},
			OnStoppedLeading: func() {
				// We lost the lock.
				klog.InfoS("leaderelection lock was lost")
				m.Stop()
			},
		},
	}
	leaderElector, err := leaderelection.NewLeaderElector(config)
	if err != nil {
		klog.ErrorS(err, "couldn't create leader elector")
		m.Stop()
	}

	leaderElector.Run(ctx)
}

// Filter annotations by namespace. i.e. adds the possibly missing default namespace for annotations
func (m *nfdMaster) filterFeatureAnnotations(annotations map[string]string) map[string]string {
	outAnnotations := make(map[string]string)

	for annotation, value := range annotations {
		// Check annotation namespace, filter out if ns is not whitelisted
		err := validate.Annotation(annotation, value)
		if err != nil {
			klog.ErrorS(err, "ignoring annotation", "annotationKey", annotation, "annotationValue", value)
			continue
		}

		outAnnotations[annotation] = value
	}

	if len(outAnnotations) > 0 && m.config.Restrictions.DisableAnnotations {
		klog.V(2).InfoS("node annotations are disabled in configuration (restrictions.disableAnnotations=true)")
		outAnnotations = map[string]string{}
	}

	return outAnnotations
}

func getNode(cli k8sclient.Interface, nodeName string) (*corev1.Node, error) {
	return cli.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
}

func getNodeFeatureGroup(cli nfdclientset.Interface, namespace, name string) (*nfdv1alpha1.NodeFeatureGroup, error) {
	return cli.NfdV1alpha1().NodeFeatureGroups(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func getNodes(cli k8sclient.Interface) (*corev1.NodeList, error) {
	return cli.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
}

func patchNode(cli k8sclient.Interface, nodeName string, patches []utils.JsonPatch, subresources ...string) error {
	if len(patches) == 0 {
		return nil
	}
	data, err := json.Marshal(patches)
	if err == nil {
		_, err = cli.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, data, metav1.PatchOptions{}, subresources...)
	}
	return err
}

func patchNodeStatus(cli k8sclient.Interface, nodeName string, patches []utils.JsonPatch) error {
	return patchNode(cli, nodeName, patches, "status")
}
