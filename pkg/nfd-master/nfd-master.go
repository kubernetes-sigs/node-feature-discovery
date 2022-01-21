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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	topologypb "sigs.k8s.io/node-feature-discovery/pkg/topologyupdater"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// FeatureLabelNs is the namespace for feature labels
	FeatureLabelNs = "feature.node.kubernetes.io"

	// FeatureLabelSubNsSuffix is the suffix for allowed feature label sub-namespaces
	FeatureLabelSubNsSuffix = "." + FeatureLabelNs

	// ProfileLabelNs is the namespace for profile labels
	ProfileLabelNs = "profile.node.kubernetes.io"

	// ProfileLabelSubNsSuffix is the suffix for allowed profile label sub-namespaces
	ProfileLabelSubNsSuffix = "." + ProfileLabelNs

	// AnnotationNsBase namespace for all NFD-related annotations
	AnnotationNsBase = "nfd.node.kubernetes.io"

	// NFD Annotations
	extendedResourceAnnotation = "extended-resources"
	featureLabelAnnotation     = "feature-labels"
	masterVersionAnnotation    = "master.version"
	workerVersionAnnotation    = "worker.version"
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// ExtendedResources are k8s extended resources which are created from discovered features.
type ExtendedResources map[string]string

// Annotations are used for NFD-related node metadata
type Annotations map[string]string

// Args holds command line arguments
type Args struct {
	CaFile                 string
	CertFile               string
	ExtraLabelNs           utils.StringSetVal
	Instance               string
	KeyFile                string
	Kubeconfig             string
	LabelWhiteList         utils.RegexpVal
	FeatureRulesController bool
	NoPublish              bool
	Port                   int
	Prune                  bool
	VerifyNodeName         bool
	ResourceLabels         utils.StringSetVal
}

type NfdMaster interface {
	Run() error
	Stop()
	WaitForReady(time.Duration) bool
}

type nfdMaster struct {
	*nfdController

	args         Args
	nodeName     string
	annotationNs string
	server       *grpc.Server
	stop         chan struct{}
	ready        chan bool
	apihelper    apihelper.APIHelpers
	kubeconfig   *restclient.Config
}

// Create new NfdMaster server instance.
func NewNfdMaster(args *Args) (NfdMaster, error) {
	nfd := &nfdMaster{args: *args,
		nodeName: os.Getenv("NODE_NAME"),
		ready:    make(chan bool, 1),
		stop:     make(chan struct{}, 1),
	}

	if args.Instance == "" {
		nfd.annotationNs = AnnotationNsBase
	} else {
		if ok, _ := regexp.MatchString(`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`, args.Instance); !ok {
			return nfd, fmt.Errorf("invalid -instance %q: instance name "+
				"must start and end with an alphanumeric character and may only contain "+
				"alphanumerics, `-`, `_` or `.`", args.Instance)
		}
		nfd.annotationNs = args.Instance + "." + AnnotationNsBase
	}

	// Check TLS related args
	if args.CertFile != "" || args.KeyFile != "" || args.CaFile != "" {
		if args.CertFile == "" {
			return nfd, fmt.Errorf("-cert-file needs to be specified alongside -key-file and -ca-file")
		}
		if args.KeyFile == "" {
			return nfd, fmt.Errorf("-key-file needs to be specified alongside -cert-file and -ca-file")
		}
		if args.CaFile == "" {
			return nfd, fmt.Errorf("-ca-file needs to be specified alongside -cert-file and -key-file")
		}
	}

	// Initialize Kubernetes API helpers
	if !args.NoPublish {
		kubeconfig, err := nfd.getKubeconfig()
		if err != nil {
			return nfd, err
		}
		nfd.apihelper = apihelper.K8sHelpers{Kubeconfig: kubeconfig}
	}

	return nfd, nil
}

// Run NfdMaster server. The method returns in case of fatal errors or if Stop()
// is called.
func (m *nfdMaster) Run() error {
	klog.Infof("Node Feature Discovery Master %s", version.Get())
	if m.args.Instance != "" {
		klog.Infof("Master instance: %q", m.args.Instance)
	}
	klog.Infof("NodeName: %q", m.nodeName)

	if m.args.Prune {
		return m.prune()
	}

	if m.args.FeatureRulesController {
		kubeconfig, err := m.getKubeconfig()
		if err != nil {
			return err
		}
		klog.Info("starting nfd LabelRule controller")
		m.nfdController = newNfdController(kubeconfig)
	}

	if !m.args.NoPublish {
		err := m.updateMasterNode()
		if err != nil {
			return fmt.Errorf("failed to update master node: %v", err)
		}
	}

	// Create server listening for TCP connections
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", m.args.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	// Notify that we're ready to accept connections
	m.ready <- true
	close(m.ready)

	serverOpts := []grpc.ServerOption{}
	tlsConfig := utils.TlsConfig{}
	// Create watcher for TLS cert files
	certWatch, err := utils.CreateFsWatcher(time.Second, m.args.CertFile, m.args.KeyFile, m.args.CaFile)
	if err != nil {
		return err
	}
	// Enable mutual TLS authentication if -cert-file, -key-file or -ca-file
	// is defined
	if m.args.CertFile != "" || m.args.KeyFile != "" || m.args.CaFile != "" {
		if err := tlsConfig.UpdateConfig(m.args.CertFile, m.args.KeyFile, m.args.CaFile); err != nil {
			return err
		}

		tlsConfig := &tls.Config{GetConfigForClient: tlsConfig.GetConfig}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	m.server = grpc.NewServer(serverOpts...)
	pb.RegisterLabelerServer(m.server, m)
	grpc_health_v1.RegisterHealthServer(m.server, health.NewServer())
	topologypb.RegisterNodeTopologyServer(m.server, m)
	klog.Infof("gRPC server serving on port: %d", m.args.Port)

	// Run gRPC server
	grpcErr := make(chan error, 1)
	go func() {
		defer lis.Close()
		grpcErr <- m.server.Serve(lis)
	}()

	// NFD-Master main event loop
	for {
		select {
		case <-certWatch.Events:
			klog.Infof("reloading TLS certificates")
			if err := tlsConfig.UpdateConfig(m.args.CertFile, m.args.KeyFile, m.args.CaFile); err != nil {
				return err
			}

		case err := <-grpcErr:
			if err != nil {
				return fmt.Errorf("gRPC server exited with an error: %v", err)
			}
			klog.Infof("gRPC server stopped")

		case <-m.stop:
			klog.Infof("shutting down nfd-master")
			certWatch.Close()
			return nil
		}
	}
}

// Stop NfdMaster
func (m *nfdMaster) Stop() {
	m.server.GracefulStop()

	if m.nfdController != nil {
		m.nfdController.stop()
	}

	select {
	case m.stop <- struct{}{}:
	default:
	}
}

// Wait until NfdMaster is able able to accept connections.
func (m *nfdMaster) WaitForReady(timeout time.Duration) bool {
	select {
	case ready, ok := <-m.ready:
		// Ready if the flag is true or the channel has been closed
		if ready || !ok {
			return true
		}
	case <-time.After(timeout):
		return false
	}
	// We should never end-up here
	return false
}

// Prune erases all NFD related properties from the node objects of the cluster.
func (m *nfdMaster) prune() error {
	cli, err := m.apihelper.GetClient()
	if err != nil {
		return err
	}

	nodes, err := m.apihelper.GetNodes(cli)
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		klog.Infof("pruning node %q...", node.Name)

		// Prune labels and extended resources
		err := m.updateNodeFeatures(node.Name, Labels{}, Annotations{}, ExtendedResources{})
		if err != nil {
			return fmt.Errorf("failed to prune labels from node %q: %v", node.Name, err)
		}

		// Prune annotations
		node, err := m.apihelper.GetNode(cli, node.Name)
		if err != nil {
			return err
		}
		for a := range node.Annotations {
			if strings.HasPrefix(a, m.annotationNs) {
				delete(node.Annotations, a)
			}
		}
		err = m.apihelper.UpdateNode(cli, node)
		if err != nil {
			return fmt.Errorf("failed to prune annotations from node %q: %v", node.Name, err)
		}
	}
	return nil
}

// Advertise NFD master information
func (m *nfdMaster) updateMasterNode() error {
	cli, err := m.apihelper.GetClient()
	if err != nil {
		return err
	}
	node, err := m.apihelper.GetNode(cli, m.nodeName)
	if err != nil {
		return err
	}

	// Advertise NFD version as an annotation
	p := createPatches(nil,
		node.Annotations,
		Annotations{m.annotationName(masterVersionAnnotation): version.Get()},
		"/metadata/annotations")
	err = m.apihelper.PatchNode(cli, node.Name, p)
	if err != nil {
		return fmt.Errorf("failed to patch node annotations: %v", err)
	}

	return nil
}

// Filter labels by namespace and name whitelist, and, turn selected labels
// into extended resources. This function also handles proper namespacing of
// labels and ERs, i.e. adds the possibly missing default namespace for labels
// arriving through the gRPC API.
func filterFeatureLabels(labels Labels, extraLabelNs map[string]struct{}, labelWhiteList regexp.Regexp, extendedResourceNames map[string]struct{}) (Labels, ExtendedResources) {
	outLabels := Labels{}

	for label, value := range labels {
		// Add possibly missing default ns
		label := addNs(label, FeatureLabelNs)

		ns, name := splitNs(label)

		// Check label namespace, filter out if ns is not whitelisted
		if ns != FeatureLabelNs && ns != ProfileLabelNs &&
			!strings.HasSuffix(ns, FeatureLabelSubNsSuffix) && !strings.HasSuffix(ns, ProfileLabelSubNsSuffix) {
			if _, ok := extraLabelNs[ns]; !ok {
				klog.Errorf("Namespace %q is not allowed. Ignoring label %q\n", ns, label)
				continue
			}
		}

		// Skip if label doesn't match labelWhiteList
		if !labelWhiteList.MatchString(name) {
			klog.Errorf("%s (%s) does not match the whitelist (%s) and will not be published.", name, label, labelWhiteList.String())
			continue
		}
		outLabels[label] = value
	}

	// Remove labels which are intended to be extended resources
	extendedResources := ExtendedResources{}
	for extendedResourceName := range extendedResourceNames {
		// Add possibly missing default ns
		extendedResourceName = addNs(extendedResourceName, FeatureLabelNs)
		if value, ok := outLabels[extendedResourceName]; ok {
			if _, err := strconv.Atoi(value); err != nil {
				klog.Errorf("bad label value (%s: %s) encountered for extended resource: %s", extendedResourceName, value, err.Error())
				continue // non-numeric label can't be used
			}

			extendedResources[extendedResourceName] = value
			delete(outLabels, extendedResourceName)
		}
	}

	return outLabels, extendedResources
}

func verifyNodeName(cert *x509.Certificate, nodeName string) error {
	if cert.Subject.CommonName == nodeName {
		return nil
	}

	err := cert.VerifyHostname(nodeName)
	if err != nil {
		return fmt.Errorf("Certificate %q not valid for node %q: %v", cert.Subject.CommonName, nodeName, err)
	}
	return nil
}

// SetLabels implements LabelerServer
func (m *nfdMaster) SetLabels(c context.Context, r *pb.SetLabelsRequest) (*pb.SetLabelsReply, error) {
	err := authorizeClient(c, m.args.VerifyNodeName, r.NodeName)
	if err != nil {
		return &pb.SetLabelsReply{}, err
	}
	switch {
	case klog.V(4).Enabled():
		utils.KlogDump(3, "REQUEST", "  ", r)
	case klog.V(1).Enabled():
		klog.Infof("REQUEST Node: %q NFD-version: %q Labels: %s", r.NodeName, r.NfdVersion, r.Labels)
	default:
		klog.Infof("received labeling request for node %q", r.NodeName)
	}

	// Mix in CR-originated labels
	rawLabels := make(map[string]string)
	if r.Labels != nil {
		// NOTE: we effectively mangle the request struct by not creating a deep copy of the map
		rawLabels = r.Labels
	}
	for k, v := range m.crLabels(r) {
		rawLabels[k] = v
	}

	labels, extendedResources := filterFeatureLabels(rawLabels, m.args.ExtraLabelNs, m.args.LabelWhiteList.Regexp, m.args.ResourceLabels)

	if !m.args.NoPublish {
		// Advertise NFD worker version as an annotation
		annotations := Annotations{m.annotationName(workerVersionAnnotation): r.NfdVersion}

		err := m.updateNodeFeatures(r.NodeName, labels, annotations, extendedResources)
		if err != nil {
			klog.Errorf("failed to advertise labels: %v", err)
			return &pb.SetLabelsReply{}, err
		}
	}
	return &pb.SetLabelsReply{}, nil
}

func authorizeClient(c context.Context, checkNodeName bool, nodeName string) error {
	if checkNodeName {
		// Client authorization.
		// Check that the node name matches the CN from the TLS cert
		client, ok := peer.FromContext(c)
		if !ok {
			klog.Errorf("gRPC request error: failed to get peer (client)")
			return fmt.Errorf("failed to get peer (client)")
		}
		tlsAuth, ok := client.AuthInfo.(credentials.TLSInfo)
		if !ok {
			klog.Errorf("gRPC request error: incorrect client credentials from '%v'", client.Addr)
			return fmt.Errorf("incorrect client credentials")
		}
		if len(tlsAuth.State.VerifiedChains) == 0 || len(tlsAuth.State.VerifiedChains[0]) == 0 {
			klog.Errorf("gRPC request error: client certificate verification for '%v' failed", client.Addr)
			return fmt.Errorf("client certificate verification failed")
		}

		err := verifyNodeName(tlsAuth.State.VerifiedChains[0][0], nodeName)
		if err != nil {
			klog.Errorf("gRPC request error: authorization for %v failed: %v", client.Addr, err)
			return err
		}
	}
	return nil
}

func (m *nfdMaster) UpdateNodeTopology(c context.Context, r *topologypb.NodeTopologyRequest) (*topologypb.NodeTopologyResponse, error) {
	err := authorizeClient(c, m.args.VerifyNodeName, r.NodeName)
	if err != nil {
		return &topologypb.NodeTopologyResponse{}, err
	}
	if klog.V(1).Enabled() {
		klog.Infof("REQUEST Node: %s NFD-version: %s Topology Policy: %s", r.NodeName, r.NfdVersion, r.TopologyPolicies)
		utils.KlogDump(1, "Zones received:", "  ", r.Zones)
	} else {
		klog.Infof("received CR updation request for node %q", r.NodeName)
	}
	if !m.args.NoPublish {
		err := m.updateCR(r.NodeName, r.TopologyPolicies, r.Zones)
		if err != nil {
			klog.Errorf("failed to advertise NodeResourceTopology: %w", err)
			return &topologypb.NodeTopologyResponse{}, err
		}
	}
	return &topologypb.NodeTopologyResponse{}, nil
}

func (m *nfdMaster) crLabels(r *pb.SetLabelsRequest) map[string]string {
	if m.nfdController == nil {
		return nil
	}

	l := make(map[string]string)
	ruleSpecs, err := m.nfdController.lister.List(labels.Everything())
	sort.Slice(ruleSpecs, func(i, j int) bool {
		return ruleSpecs[i].Name < ruleSpecs[j].Name
	})

	if err != nil {
		klog.Errorf("failed to list LabelRule resources: %w", err)
		return nil
	}

	// Process all rule CRs
	for _, spec := range ruleSpecs {
		switch {
		case klog.V(3).Enabled():
			h := fmt.Sprintf("executing LabelRule \"%s/%s\":", spec.ObjectMeta.Namespace, spec.ObjectMeta.Name)
			utils.KlogDump(3, h, "  ", spec.Spec)
		case klog.V(1).Enabled():
			klog.Infof("executing LabelRule \"%s/%s\"", spec.ObjectMeta.Namespace, spec.ObjectMeta.Name)
		}
		for _, rule := range spec.Spec.Rules {
			ruleOut, err := rule.Execute(r.Features)
			if err != nil {
				klog.Errorf("failed to process Rule %q: %w", rule.Name, err)
				continue
			}

			for k, v := range ruleOut.Labels {
				l[k] = v
			}

			// Feed back rule output to features map for subsequent rules to match
			feature.InsertFeatureValues(r.Features, nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Labels)
			feature.InsertFeatureValues(r.Features, nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Vars)
		}
	}

	return l
}

// updateNodeFeatures ensures the Kubernetes node object is up to date,
// creating new labels and extended resources where necessary and removing
// outdated ones. Also updates the corresponding annotations.
func (m *nfdMaster) updateNodeFeatures(nodeName string, labels Labels, annotations Annotations, extendedResources ExtendedResources) error {
	cli, err := m.apihelper.GetClient()
	if err != nil {
		return err
	}

	// Get the worker node object
	node, err := m.apihelper.GetNode(cli, nodeName)
	if err != nil {
		return err
	}

	// Store names of labels in an annotation
	labelKeys := make([]string, 0, len(labels))
	for key := range labels {
		// Drop the ns part for labels in the default ns
		labelKeys = append(labelKeys, strings.TrimPrefix(key, FeatureLabelNs+"/"))
	}
	sort.Strings(labelKeys)
	annotations[m.annotationName(featureLabelAnnotation)] = strings.Join(labelKeys, ",")

	// Store names of extended resources in an annotation
	extendedResourceKeys := make([]string, 0, len(extendedResources))
	for key := range extendedResources {
		// Drop the ns part if in the default ns
		extendedResourceKeys = append(extendedResourceKeys, strings.TrimPrefix(key, FeatureLabelNs+"/"))
	}
	sort.Strings(extendedResourceKeys)
	annotations[m.annotationName(extendedResourceAnnotation)] = strings.Join(extendedResourceKeys, ",")

	// Create JSON patches for changes in labels and annotations
	oldLabels := stringToNsNames(node.Annotations[m.annotationName(featureLabelAnnotation)], FeatureLabelNs)
	patches := createPatches(oldLabels, node.Labels, labels, "/metadata/labels")
	patches = append(patches, createPatches(nil, node.Annotations, annotations, "/metadata/annotations")...)

	// Also, remove all labels with the old prefix, and the old version label
	patches = append(patches, removeLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/nfd")...)
	patches = append(patches, removeLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/node-feature-discovery")...)

	// Patch the node object in the apiserver
	err = m.apihelper.PatchNode(cli, node.Name, patches)
	if err != nil {
		return fmt.Errorf("error while patching node object: %v", err)
	}

	// patch node status with extended resource changes
	patches = m.createExtendedResourcePatches(node, extendedResources)
	err = m.apihelper.PatchNodeStatus(cli, node.Name, patches)
	if err != nil {
		return fmt.Errorf("error while patching extended resources: %v", err)
	}

	return err
}

func (m *nfdMaster) annotationName(name string) string {
	return path.Join(m.annotationNs, name)
}

func (m *nfdMaster) getKubeconfig() (*restclient.Config, error) {
	var err error
	if m.kubeconfig == nil {
		m.kubeconfig, err = apihelper.GetKubeconfig(m.args.Kubeconfig)
	}
	return m.kubeconfig, err
}

// Remove any labels having the given prefix
func removeLabelsWithPrefix(n *api.Node, search string) []apihelper.JsonPatch {
	var p []apihelper.JsonPatch

	for k := range n.Labels {
		if strings.HasPrefix(k, search) {
			p = append(p, apihelper.NewJsonPatch("remove", "/metadata/labels", k, ""))
		}
	}

	return p
}

// createPatches is a generic helper that returns json patch operations to perform
func createPatches(removeKeys []string, oldItems map[string]string, newItems map[string]string, jsonPath string) []apihelper.JsonPatch {
	patches := []apihelper.JsonPatch{}

	// Determine items to remove
	for _, key := range removeKeys {
		if _, ok := oldItems[key]; ok {
			if _, ok := newItems[key]; !ok {
				patches = append(patches, apihelper.NewJsonPatch("remove", jsonPath, key, ""))
			}
		}
	}

	// Determine items to add or replace
	for key, newVal := range newItems {
		if oldVal, ok := oldItems[key]; ok {
			if newVal != oldVal {
				patches = append(patches, apihelper.NewJsonPatch("replace", jsonPath, key, newVal))
			}
		} else {
			patches = append(patches, apihelper.NewJsonPatch("add", jsonPath, key, newVal))
		}
	}

	return patches
}

// createExtendedResourcePatches returns a slice of operations to perform on
// the node status
func (m *nfdMaster) createExtendedResourcePatches(n *api.Node, extendedResources ExtendedResources) []apihelper.JsonPatch {
	patches := []apihelper.JsonPatch{}

	// Form a list of namespaced resource names managed by us
	oldResources := stringToNsNames(n.Annotations[m.annotationName(extendedResourceAnnotation)], FeatureLabelNs)

	// figure out which resources to remove
	for _, resource := range oldResources {
		if _, ok := n.Status.Capacity[api.ResourceName(resource)]; ok {
			// check if the ext resource is still needed
			if _, extResNeeded := extendedResources[resource]; !extResNeeded {
				patches = append(patches, apihelper.NewJsonPatch("remove", "/status/capacity", resource, ""))
				patches = append(patches, apihelper.NewJsonPatch("remove", "/status/allocatable", resource, ""))
			}
		}
	}

	// figure out which resources to replace and which to add
	for resource, value := range extendedResources {
		// check if the extended resource already exists with the same capacity in the node
		if quantity, ok := n.Status.Capacity[api.ResourceName(resource)]; ok {
			val, _ := quantity.AsInt64()
			if strconv.FormatInt(val, 10) != value {
				patches = append(patches, apihelper.NewJsonPatch("replace", "/status/capacity", resource, value))
				patches = append(patches, apihelper.NewJsonPatch("replace", "/status/allocatable", resource, value))
			}
		} else {
			patches = append(patches, apihelper.NewJsonPatch("add", "/status/capacity", resource, value))
			// "allocatable" gets added implicitly after adding to capacity
		}
	}

	return patches
}

// addNs adds a namespace if one isn't already found from src string
func addNs(src string, nsToAdd string) string {
	if strings.Contains(src, "/") {
		return src
	}
	return path.Join(nsToAdd, src)
}

// splitNs splits a name into its namespace and name parts
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

func modifyCR(topoUpdaterZones []*v1alpha1.Zone) []v1alpha1.Zone {
	zones := make([]v1alpha1.Zone, len(topoUpdaterZones))
	// TODO: Avoid copying of data to allow returning the zone info
	// directly in a compatible data type (i.e. []*v1alpha1.Zone).
	for i, zone := range topoUpdaterZones {
		zones[i] = v1alpha1.Zone{
			Name:      zone.Name,
			Type:      zone.Type,
			Parent:    zone.Parent,
			Costs:     zone.Costs,
			Resources: zone.Resources,
		}
	}
	return zones
}

func (m *nfdMaster) updateCR(hostname string, tmpolicy []string, topoUpdaterZones []*v1alpha1.Zone) error {
	cli, err := m.apihelper.GetTopologyClient()
	if err != nil {
		return err
	}

	zones := modifyCR(topoUpdaterZones)

	nrt, err := cli.TopologyV1alpha1().NodeResourceTopologies().Get(context.TODO(), hostname, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nrtNew := v1alpha1.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name: hostname,
			},
			Zones:            zones,
			TopologyPolicies: tmpolicy,
		}

		_, err := cli.TopologyV1alpha1().NodeResourceTopologies().Create(context.TODO(), &nrtNew, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create v1alpha1.NodeResourceTopology!:%w", err)
		}
		return nil
	} else if err != nil {
		return err
	}

	nrtMutated := nrt.DeepCopy()
	nrtMutated.Zones = zones

	nrtUpdated, err := cli.TopologyV1alpha1().NodeResourceTopologies().Update(context.TODO(), nrtMutated, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update v1alpha1.NodeResourceTopology!:%w", err)
	}
	utils.KlogDump(2, "CR instance updated resTopo:", "  ", nrtUpdated)
	return nil
}
