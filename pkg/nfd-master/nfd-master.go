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
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	label "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	controller "k8s.io/kubernetes/pkg/controller"
	taintutils "k8s.io/kubernetes/pkg/util/taints"

	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// ExtendedResources are k8s extended resources which are created from discovered features.
type ExtendedResources map[string]string

// Annotations are used for NFD-related node metadata
type Annotations map[string]string

// Args holds command line arguments
type Args struct {
	CaFile               string
	CertFile             string
	ExtraLabelNs         utils.StringSetVal
	Instance             string
	KeyFile              string
	Kubeconfig           string
	LabelWhiteList       utils.RegexpVal
	CrdController        bool
	EnableNodeFeatureApi bool
	NoPublish            bool
	EnableTaints         bool
	Port                 int
	Prune                bool
	VerifyNodeName       bool
	ResourceLabels       utils.StringSetVal
}

type NfdMaster interface {
	Run() error
	Stop()
	WaitForReady(time.Duration) bool
}

type nfdMaster struct {
	*nfdController

	args       Args
	namespace  string
	nodeName   string
	server     *grpc.Server
	stop       chan struct{}
	ready      chan bool
	apihelper  apihelper.APIHelpers
	kubeconfig *restclient.Config
}

// NewNfdMaster creates a new NfdMaster server instance.
func NewNfdMaster(args *Args) (NfdMaster, error) {
	nfd := &nfdMaster{args: *args,
		nodeName:  utils.NodeName(),
		namespace: utils.GetKubernetesNamespace(),
		ready:     make(chan bool, 1),
		stop:      make(chan struct{}, 1),
	}

	if args.Instance != "" {
		if ok, _ := regexp.MatchString(`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`, args.Instance); !ok {
			return nfd, fmt.Errorf("invalid -instance %q: instance name "+
				"must start and end with an alphanumeric character and may only contain "+
				"alphanumerics, `-`, `_` or `.`", args.Instance)
		}
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
	klog.Infof("Kubernetes namespace: %q", m.namespace)

	if m.args.Prune {
		return m.prune()
	}

	if m.args.CrdController {
		kubeconfig, err := m.getKubeconfig()
		if err != nil {
			return err
		}
		klog.Info("starting nfd api controller")
		m.nfdController, err = newNfdController(kubeconfig, !m.args.EnableNodeFeatureApi)
		if err != nil {
			return fmt.Errorf("failed to initialize CRD controller: %w", err)
		}
	}

	if !m.args.NoPublish {
		err := m.updateMasterNode()
		if err != nil {
			return fmt.Errorf("failed to update master node: %v", err)
		}
	}

	// Run gRPC server
	grpcErr := make(chan error, 1)
	if !m.args.EnableNodeFeatureApi {
		go m.runGrpcServer(grpcErr)
	}

	// Run updater that handles events from the nfd CRD API.
	if m.nfdController != nil {
		go m.nfdAPIUpdateHandler()
	}

	// Notify that we're ready to accept connections
	m.ready <- true
	close(m.ready)

	// NFD-Master main event loop
	for {
		select {
		case err := <-grpcErr:
			return fmt.Errorf("error in serving gRPC: %w", err)

		case <-m.stop:
			klog.Infof("shutting down nfd-master")
			return nil
		}
	}
}

func (m *nfdMaster) runGrpcServer(errChan chan<- error) {
	// Create server listening for TCP connections
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", m.args.Port))
	if err != nil {
		errChan <- fmt.Errorf("failed to listen: %v", err)
		return
	}

	serverOpts := []grpc.ServerOption{}
	tlsConfig := utils.TlsConfig{}
	// Create watcher for TLS cert files
	certWatch, err := utils.CreateFsWatcher(time.Second, m.args.CertFile, m.args.KeyFile, m.args.CaFile)
	if err != nil {
		errChan <- err
		return
	}
	// Enable mutual TLS authentication if -cert-file, -key-file or -ca-file
	// is defined
	if m.args.CertFile != "" || m.args.KeyFile != "" || m.args.CaFile != "" {
		if err := tlsConfig.UpdateConfig(m.args.CertFile, m.args.KeyFile, m.args.CaFile); err != nil {
			errChan <- err
			return
		}

		tlsConfig := &tls.Config{GetConfigForClient: tlsConfig.GetConfig}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	m.server = grpc.NewServer(serverOpts...)
	pb.RegisterLabelerServer(m.server, m)
	grpc_health_v1.RegisterHealthServer(m.server, health.NewServer())
	klog.Infof("gRPC server serving on port: %d", m.args.Port)

	// Run gRPC server
	grpcErr := make(chan error, 1)
	go func() {
		defer lis.Close()
		grpcErr <- m.server.Serve(lis)
	}()

	for {
		select {
		case <-certWatch.Events:
			klog.Infof("reloading TLS certificates")
			if err := tlsConfig.UpdateConfig(m.args.CertFile, m.args.KeyFile, m.args.CaFile); err != nil {
				errChan <- err
			}

		case err := <-grpcErr:
			if err != nil {
				errChan <- fmt.Errorf("gRPC server exited with an error: %v", err)
			}
			klog.Infof("gRPC server stopped")
		}
	}
}

// nfdAPIUpdateHandler handles events from the nfd API controller.
func (m *nfdMaster) nfdAPIUpdateHandler() {
	// We want to unconditionally update all nodes at startup if gRPC is
	// disabled (i.e. NodeFeature API is enabled)
	updateAll := m.args.EnableNodeFeatureApi
	updateNodes := make(map[string]struct{})
	rateLimit := time.After(time.Second)
	for {
		select {
		case <-m.nfdController.updateAllNodesChan:
			updateAll = true
		case nodeName := <-m.nfdController.updateOneNodeChan:
			updateNodes[nodeName] = struct{}{}
		case <-rateLimit:
			// Check what we need to do
			// TODO: we might want to update multiple nodes in parallel
			if updateAll {
				if err := m.nfdAPIUpdateAllNodes(); err != nil {
					klog.Error(err)
				}
			} else {
				for nodeName := range updateNodes {
					if err := m.nfdAPIUpdateOneNode(nodeName); err != nil {
						klog.Error(err)
					}
				}
			}

			// Reset "work queue" and timer
			updateAll = false
			updateNodes = make(map[string]struct{})
			rateLimit = time.After(time.Second)
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
		err := m.updateNodeObject(cli, node.Name, Labels{}, Annotations{}, ExtendedResources{}, []corev1.Taint{})
		if err != nil {
			return fmt.Errorf("failed to prune node %q: %v", node.Name, err)
		}

		// Prune annotations
		node, err := m.apihelper.GetNode(cli, node.Name)
		if err != nil {
			return err
		}
		for a := range node.Annotations {
			if strings.HasPrefix(a, m.instanceAnnotation(nfdv1alpha1.AnnotationNs)) {
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
		Annotations{m.instanceAnnotation(nfdv1alpha1.MasterVersionAnnotation): version.Get()},
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
		label := addNs(label, nfdv1alpha1.FeatureLabelNs)

		ns, name := splitNs(label)

		// Check label namespace, filter out if ns is not whitelisted
		if ns != nfdv1alpha1.FeatureLabelNs && ns != nfdv1alpha1.ProfileLabelNs &&
			!strings.HasSuffix(ns, nfdv1alpha1.FeatureLabelSubNsSuffix) && !strings.HasSuffix(ns, nfdv1alpha1.ProfileLabelSubNsSuffix) {
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
		extendedResourceName = addNs(extendedResourceName, nfdv1alpha1.FeatureLabelNs)
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
		return fmt.Errorf("certificate %q not valid for node %q: %v", cert.Subject.CommonName, nodeName, err)
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

	if !m.args.NoPublish {
		cli, err := m.apihelper.GetClient()
		if err != nil {
			return &pb.SetLabelsReply{}, err
		}

		// Advertise NFD worker version as an annotation
		annotations := Annotations{m.instanceAnnotation(nfdv1alpha1.WorkerVersionAnnotation): r.NfdVersion}

		// Create labels et al
		if err := m.refreshNodeFeatures(cli, r.NodeName, annotations, r.GetLabels(), r.GetFeatures()); err != nil {
			return &pb.SetLabelsReply{}, err
		}
	}
	return &pb.SetLabelsReply{}, nil
}

func (m *nfdMaster) nfdAPIUpdateAllNodes() error {
	klog.Info("will process all nodes in the cluster")

	cli, err := m.apihelper.GetClient()
	if err != nil {
		return err
	}

	nodes, err := m.apihelper.GetNodes(cli)
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		if err := m.nfdAPIUpdateOneNode(node.Name); err != nil {
			return err
		}
	}

	return nil
}

func (m *nfdMaster) nfdAPIUpdateOneNode(nodeName string) error {
	sel := labels.SelectorFromSet(labels.Set{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodeName})
	objs, err := m.nfdController.featureLister.List(sel)
	if err != nil {
		return fmt.Errorf("failed to get NodeFeature resources for node %q: %w", nodeName, err)
	}

	// Sort our objects
	sort.Slice(objs, func(i, j int) bool {
		// Objects in our nfd namespace gets into the beginning of the list
		if objs[i].Namespace == m.namespace && objs[j].Namespace != m.namespace {
			return true
		}
		if objs[i].Namespace != m.namespace && objs[j].Namespace == m.namespace {
			return false
		}
		// After the nfd namespace, sort objects by their name
		if objs[i].Name != objs[j].Name {
			return objs[i].Name < objs[j].Name
		}
		// Objects with the same name are sorted by their namespace
		return objs[i].Namespace < objs[j].Namespace
	})

	if m.args.NoPublish {
		return nil
	}

	klog.V(1).Infof("processing node %q, initiated by NodeFeature API", nodeName)

	features := &nfdv1alpha1.NodeFeatureSpec{}
	annotations := Annotations{}

	if len(objs) > 0 {
		// Merge in features
		//
		// NOTE: changing the rule api to support handle multiple objects instead
		// of merging would probably perform better with lot less data to copy.
		features = objs[0].Spec.DeepCopy()
		for _, o := range objs[1:] {
			o.Spec.MergeInto(features)
		}

		utils.KlogDump(4, "Composite NodeFeatureSpec after merge:", "  ", features)

		if objs[0].Namespace == m.namespace && objs[0].Name == nodeName {
			// This is the one created by nfd-worker
			if v := objs[0].Annotations[nfdv1alpha1.WorkerVersionAnnotation]; v != "" {
				annotations[nfdv1alpha1.WorkerVersionAnnotation] = v
			}
		}
	}

	// Update node labels et al. This may also mean removing all NFD-owned
	// labels (et al.), for example  in the case no NodeFeature objects are
	// present.
	cli, err := m.apihelper.GetClient()
	if err != nil {
		return err
	}
	if err := m.refreshNodeFeatures(cli, nodeName, annotations, features.Labels, &features.Features); err != nil {
		return err
	}

	return nil
}

func (m *nfdMaster) refreshNodeFeatures(cli *kubernetes.Clientset, nodeName string, annotations, labels map[string]string, features *nfdv1alpha1.Features) error {
	if labels == nil {
		labels = make(map[string]string)
	}

	crLabels, crTaints := m.processNodeFeatureRule(features)

	// Mix in CR-originated labels
	for k, v := range crLabels {
		labels[k] = v
	}

	labels, extendedResources := filterFeatureLabels(labels, m.args.ExtraLabelNs, m.args.LabelWhiteList.Regexp, m.args.ResourceLabels)

	var taints []corev1.Taint
	if m.args.EnableTaints {
		taints = crTaints
	}

	err := m.updateNodeObject(cli, nodeName, labels, annotations, extendedResources, taints)
	if err != nil {
		klog.Errorf("failed to update node %q: %v", nodeName, err)
		return err
	}

	return nil
}

// setTaints sets node taints and annotations based on the taints passed via
// nodeFeatureRule custom resorce. If empty list of taints is passed, currently
// NFD owned taints and annotations are removed from the node.
func (m *nfdMaster) setTaints(cli *kubernetes.Clientset, taints []corev1.Taint, nodeName string) error {
	// Fetch the node object.
	node, err := m.apihelper.GetNode(cli, nodeName)
	if err != nil {
		return err
	}

	// De-serialize the taints annotation into corev1.Taint type for comparision below.
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
			klog.V(1).Infof("taint %q already deleted from node", taintToRemove.ToString())
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
		err = controller.PatchNodeTaints(context.TODO(), cli, nodeName, node, newNode)
		if err != nil {
			return fmt.Errorf("failed to patch the node %v", node.Name)
		}
		klog.Infof("updated node %q taints", nodeName)
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

	patches := createPatches([]string{nfdv1alpha1.NodeTaintsAnnotation}, node.Annotations, newAnnotations, "/metadata/annotations")
	if len(patches) > 0 {
		err = m.apihelper.PatchNode(cli, node.Name, patches)
		if err != nil {
			return fmt.Errorf("error while patching node object: %v", err)
		}
		klog.V(1).Infof("patched node %q annotations for taints", nodeName)
	}
	return nil
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

func (m *nfdMaster) processNodeFeatureRule(features *nfdv1alpha1.Features) (map[string]string, []corev1.Taint) {
	if m.nfdController == nil {
		return nil, nil
	}

	labels := make(map[string]string)
	var taints []corev1.Taint
	ruleSpecs, err := m.nfdController.ruleLister.List(label.Everything())
	sort.Slice(ruleSpecs, func(i, j int) bool {
		return ruleSpecs[i].Name < ruleSpecs[j].Name
	})

	if err != nil {
		klog.Errorf("failed to list NodeFeatureRule resources: %v", err)
		return nil, nil
	}

	// Process all rule CRs
	for _, spec := range ruleSpecs {
		switch {
		case klog.V(3).Enabled():
			h := fmt.Sprintf("executing NodeFeatureRule %q:", spec.Name)
			utils.KlogDump(3, h, "  ", spec.Spec)
		case klog.V(1).Enabled():
			klog.Infof("executing NodeFeatureRule %q", spec.Name)
		}
		for _, rule := range spec.Spec.Rules {
			ruleOut, err := rule.Execute(features)
			if err != nil {
				klog.Errorf("failed to process Rule %q: %v", rule.Name, err)
				continue
			}
			taints = append(taints, ruleOut.Taints...)
			for k, v := range ruleOut.Labels {
				labels[k] = v
			}

			// Feed back rule output to features map for subsequent rules to match
			features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Labels)
			features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Vars)
		}
	}

	return labels, taints
}

// updateNodeObject ensures the Kubernetes node object is up to date,
// creating new labels and extended resources where necessary and removing
// outdated ones. Also updates the corresponding annotations.
func (m *nfdMaster) updateNodeObject(cli *kubernetes.Clientset, nodeName string, labels Labels, annotations Annotations, extendedResources ExtendedResources, taints []corev1.Taint) error {
	if cli == nil {
		return fmt.Errorf("no client is passed, client:  %v", cli)
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
		labelKeys = append(labelKeys, strings.TrimPrefix(key, nfdv1alpha1.FeatureLabelNs+"/"))
	}
	sort.Strings(labelKeys)
	annotations[m.instanceAnnotation(nfdv1alpha1.FeatureLabelsAnnotation)] = strings.Join(labelKeys, ",")

	// Store names of extended resources in an annotation
	extendedResourceKeys := make([]string, 0, len(extendedResources))
	for key := range extendedResources {
		// Drop the ns part if in the default ns
		extendedResourceKeys = append(extendedResourceKeys, strings.TrimPrefix(key, nfdv1alpha1.FeatureLabelNs+"/"))
	}
	sort.Strings(extendedResourceKeys)
	annotations[m.instanceAnnotation(nfdv1alpha1.ExtendedResourceAnnotation)] = strings.Join(extendedResourceKeys, ",")

	// Create JSON patches for changes in labels and annotations
	oldLabels := stringToNsNames(node.Annotations[m.instanceAnnotation(nfdv1alpha1.FeatureLabelsAnnotation)], nfdv1alpha1.FeatureLabelNs)
	patches := createPatches(oldLabels, node.Labels, labels, "/metadata/labels")
	patches = append(patches, createPatches(nil, node.Annotations, annotations, "/metadata/annotations")...)

	// Patch the node object in the apiserver
	err = m.apihelper.PatchNode(cli, node.Name, patches)
	if err != nil {
		return fmt.Errorf("error while patching node object: %v", err)
	}

	// patch node status with extended resource changes
	statusPatches := m.createExtendedResourcePatches(node, extendedResources)
	err = m.apihelper.PatchNodeStatus(cli, node.Name, statusPatches)
	if err != nil {
		return fmt.Errorf("error while patching extended resources: %v", err)
	}

	if len(patches) > 0 || len(statusPatches) > 0 {
		klog.Infof("node %q updated", nodeName)
	} else {
		klog.V(1).Infof("no updates to node %q", nodeName)
	}

	// Set taints
	err = m.setTaints(cli, taints, node.Name)
	if err != nil {
		return err
	}

	return err
}

func (m *nfdMaster) getKubeconfig() (*restclient.Config, error) {
	var err error
	if m.kubeconfig == nil {
		m.kubeconfig, err = apihelper.GetKubeconfig(m.args.Kubeconfig)
	}
	return m.kubeconfig, err
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
func (m *nfdMaster) createExtendedResourcePatches(n *corev1.Node, extendedResources ExtendedResources) []apihelper.JsonPatch {
	patches := []apihelper.JsonPatch{}

	// Form a list of namespaced resource names managed by us
	oldResources := stringToNsNames(n.Annotations[m.instanceAnnotation(nfdv1alpha1.ExtendedResourceAnnotation)], nfdv1alpha1.FeatureLabelNs)

	// figure out which resources to remove
	for _, resource := range oldResources {
		if _, ok := n.Status.Capacity[corev1.ResourceName(resource)]; ok {
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
		if quantity, ok := n.Status.Capacity[corev1.ResourceName(resource)]; ok {
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

func (m *nfdMaster) instanceAnnotation(name string) string {
	if m.args.Instance == "" {
		return name
	}
	return m.args.Instance + "." + name
}
