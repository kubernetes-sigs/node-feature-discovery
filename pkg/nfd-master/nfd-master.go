/*
Copyright 2019 The Kubernetes Authors.

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
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	api "k8s.io/api/core/v1"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// Namespace for feature labels
	LabelNs = "feature.node.kubernetes.io"

	// Namespace for all NFD-related annotations
	AnnotationNs = "nfd.node.kubernetes.io"

	// NFD Annotations
	extendedResourceAnnotation = AnnotationNs + "/extended-resources"
	featureLabelAnnotation     = AnnotationNs + "/feature-labels"
	masterVersionAnnotation    = AnnotationNs + "/master.version"
	workerVersionAnnotation    = AnnotationNs + "/worker.version"
)

// package loggers
var (
	stdoutLogger = log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger = log.New(os.Stderr, "", log.LstdFlags)
	nodeName     = os.Getenv("NODE_NAME")
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// ExtendedResources are k8s extended resources which are created from discovered features.
type ExtendedResources map[string]string

// Annotations are used for NFD-related node metadata
type Annotations map[string]string

// Command line arguments
type Args struct {
	CaFile         string
	CertFile       string
	ExtraLabelNs   map[string]struct{}
	KeyFile        string
	Kubeconfig     string
	LabelWhiteList *regexp.Regexp
	NoPublish      bool
	Port           int
	Prune          bool
	VerifyNodeName bool
	ResourceLabels []string
}

type NfdMaster interface {
	Run() error
	Stop()
	WaitForReady(time.Duration) bool
}

type nfdMaster struct {
	args      Args
	server    *grpc.Server
	ready     chan bool
	apihelper apihelper.APIHelpers
}

// Create new NfdMaster server instance.
func NewNfdMaster(args Args) (NfdMaster, error) {
	nfd := &nfdMaster{args: args, ready: make(chan bool, 1)}

	// Check TLS related args
	if args.CertFile != "" || args.KeyFile != "" || args.CaFile != "" {
		if args.CertFile == "" {
			return nfd, fmt.Errorf("--cert-file needs to be specified alongside --key-file and --ca-file")
		}
		if args.KeyFile == "" {
			return nfd, fmt.Errorf("--key-file needs to be specified alongside --cert-file and --ca-file")
		}
		if args.CaFile == "" {
			return nfd, fmt.Errorf("--ca-file needs to be specified alongside --cert-file and --key-file")
		}
	}

	// Initialize Kubernetes API helpers
	nfd.apihelper = apihelper.K8sHelpers{Kubeconfig: args.Kubeconfig}

	return nfd, nil
}

// Run NfdMaster server. The method returns in case of fatal errors or if Stop()
// is called.
func (m *nfdMaster) Run() error {
	stdoutLogger.Printf("Node Feature Discovery Master %s", version.Get())
	stdoutLogger.Printf("NodeName: '%s'", nodeName)

	if m.args.Prune {
		return m.prune()
	}

	if !m.args.NoPublish {
		err := updateMasterNode(m.apihelper)
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
	// Enable mutual TLS authentication if --cert-file, --key-file or --ca-file
	// is defined
	if m.args.CertFile != "" || m.args.KeyFile != "" || m.args.CaFile != "" {
		// Load cert for authenticating this server
		cert, err := tls.LoadX509KeyPair(m.args.CertFile, m.args.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load server certificate: %v", err)
		}
		// Load CA cert for client cert verification
		caCert, err := ioutil.ReadFile(m.args.CaFile)
		if err != nil {
			return fmt.Errorf("failed to read root certificate file: %v", err)
		}
		caPool := x509.NewCertPool()
		if ok := caPool.AppendCertsFromPEM(caCert); !ok {
			return fmt.Errorf("failed to add certificate from '%s'", m.args.CaFile)
		}
		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	m.server = grpc.NewServer(serverOpts...)
	pb.RegisterLabelerServer(m.server, &labelerServer{args: m.args, apiHelper: m.apihelper})
	stdoutLogger.Printf("gRPC server serving on port: %d", m.args.Port)
	return m.server.Serve(lis)
}

// Stop NfdMaster
func (m *nfdMaster) Stop() {
	m.server.Stop()
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
		stdoutLogger.Printf("pruning node %q...", node.Name)

		// Prune labels and extended resources
		err := updateNodeFeatures(m.apihelper, node.Name, Labels{}, Annotations{}, ExtendedResources{})
		if err != nil {
			return fmt.Errorf("failed to prune labels from node %q: %v", node.Name, err)
		}

		// Prune annotations
		node, err := m.apihelper.GetNode(cli, node.Name)
		if err != nil {
			return err
		}
		for a := range node.Annotations {
			if strings.HasPrefix(a, AnnotationNs) {
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
func updateMasterNode(helper apihelper.APIHelpers) error {
	cli, err := helper.GetClient()
	if err != nil {
		return err
	}
	node, err := helper.GetNode(cli, nodeName)
	if err != nil {
		return err
	}

	// Advertise NFD version as an annotation
	p := createPatches(nil, node.Annotations, Annotations{masterVersionAnnotation: version.Get()}, "/metadata/annotations")
	err = helper.PatchNode(cli, node.Name, p)
	if err != nil {
		stderrLogger.Printf("failed to patch node annotations: %v", err)
		return err
	}

	return nil
}

// Filter labels by namespace and name whitelist, and, turn selected labels
// into extended resources. This function also handles proper namespacing of
// labels and ERs, i.e. adds the possibly missing default namespace for labels
// arriving through the gRPC API.
func filterFeatureLabels(labels Labels, extraLabelNs map[string]struct{}, labelWhiteList *regexp.Regexp, extendedResourceNames []string) (Labels, ExtendedResources) {
	outLabels := Labels{}

	for label, value := range labels {
		// Add possibly missing default ns
		label := addNs(label, LabelNs)

		ns, name := splitNs(label)

		// Check label namespace, filter out if ns is not whitelisted
		if ns != LabelNs {
			if _, ok := extraLabelNs[ns]; !ok {
				stderrLogger.Printf("Namespace '%s' is not allowed. Ignoring label '%s'\n", ns, label)
				continue
			}
		}

		// Skip if label doesn't match labelWhiteList
		if !labelWhiteList.MatchString(name) {
			stderrLogger.Printf("%s (%s) does not match the whitelist (%s) and will not be published.", name, label, labelWhiteList.String())
			continue
		}
		outLabels[label] = value
	}

	// Remove labels which are intended to be extended resources
	extendedResources := ExtendedResources{}
	for _, extendedResourceName := range extendedResourceNames {
		// Add possibly missing default ns
		extendedResourceName = addNs(extendedResourceName, LabelNs)
		if value, ok := outLabels[extendedResourceName]; ok {
			if _, err := strconv.Atoi(value); err != nil {
				stderrLogger.Printf("bad label value (%s: %s) encountered for extended resource: %s", extendedResourceName, value, err.Error())
				continue // non-numeric label can't be used
			}

			extendedResources[extendedResourceName] = value
			delete(outLabels, extendedResourceName)
		}
	}

	return outLabels, extendedResources
}

// Implement LabelerServer
type labelerServer struct {
	args      Args
	apiHelper apihelper.APIHelpers
}

// Service SetLabels
func (s *labelerServer) SetLabels(c context.Context, r *pb.SetLabelsRequest) (*pb.SetLabelsReply, error) {
	if s.args.VerifyNodeName {
		// Client authorization.
		// Check that the node name matches the CN from the TLS cert
		client, ok := peer.FromContext(c)
		if !ok {
			stderrLogger.Printf("gRPC request error: failed to get peer (client)")
			return &pb.SetLabelsReply{}, fmt.Errorf("failed to get peer (client)")
		}
		tlsAuth, ok := client.AuthInfo.(credentials.TLSInfo)
		if !ok {
			stderrLogger.Printf("gRPC request error: incorrect client credentials from '%v'", client.Addr)
			return &pb.SetLabelsReply{}, fmt.Errorf("incorrect client credentials")
		}
		if len(tlsAuth.State.VerifiedChains) == 0 || len(tlsAuth.State.VerifiedChains[0]) == 0 {
			stderrLogger.Printf("gRPC request error: client certificate verification for '%v' failed", client.Addr)
			return &pb.SetLabelsReply{}, fmt.Errorf("client certificate verification failed")
		}
		cn := tlsAuth.State.VerifiedChains[0][0].Subject.CommonName
		if cn != r.NodeName {
			stderrLogger.Printf("gRPC request error: authorization for %v failed: cert valid for '%s', requested node name '%s'", client.Addr, cn, r.NodeName)
			return &pb.SetLabelsReply{}, fmt.Errorf("request authorization failed: cert valid for '%s', requested node name '%s'", cn, r.NodeName)
		}
	}
	stdoutLogger.Printf("REQUEST Node: %s NFD-version: %s Labels: %s", r.NodeName, r.NfdVersion, r.Labels)

	labels, extendedResources := filterFeatureLabels(r.Labels, s.args.ExtraLabelNs, s.args.LabelWhiteList, s.args.ResourceLabels)

	if !s.args.NoPublish {
		// Advertise NFD worker version as an annotation
		annotations := Annotations{workerVersionAnnotation: r.NfdVersion}

		err := updateNodeFeatures(s.apiHelper, r.NodeName, labels, annotations, extendedResources)
		if err != nil {
			stderrLogger.Printf("failed to advertise labels: %s", err.Error())
			return &pb.SetLabelsReply{}, err
		}
	}
	return &pb.SetLabelsReply{}, nil
}

// updateNodeFeatures ensures the Kubernetes node object is up to date,
// creating new labels and extended resources where necessary and removing
// outdated ones. Also updates the corresponding annotations.
func updateNodeFeatures(helper apihelper.APIHelpers, nodeName string, labels Labels, annotations Annotations, extendedResources ExtendedResources) error {
	cli, err := helper.GetClient()
	if err != nil {
		return err
	}

	// Get the worker node object
	node, err := helper.GetNode(cli, nodeName)
	if err != nil {
		return err
	}

	// Store names of labels in an annotation
	labelKeys := make([]string, 0, len(labels))
	for key := range labels {
		// Drop the ns part for labels in the default ns
		labelKeys = append(labelKeys, strings.TrimPrefix(key, LabelNs+"/"))
	}
	sort.Strings(labelKeys)
	annotations[featureLabelAnnotation] = strings.Join(labelKeys, ",")

	// Store names of extended resources in an annotation
	extendedResourceKeys := make([]string, 0, len(extendedResources))
	for key := range extendedResources {
		// Drop the ns part if in the default ns
		extendedResourceKeys = append(extendedResourceKeys, strings.TrimPrefix(key, LabelNs+"/"))
	}
	sort.Strings(extendedResourceKeys)
	annotations[extendedResourceAnnotation] = strings.Join(extendedResourceKeys, ",")

	// Create JSON patches for changes in labels and annotations
	oldLabels := stringToNsNames(node.Annotations[featureLabelAnnotation], LabelNs)
	patches := createPatches(oldLabels, node.Labels, labels, "/metadata/labels")
	patches = append(patches, createPatches(nil, node.Annotations, annotations, "/metadata/annotations")...)

	// Also, remove all labels with the old prefix, and the old version label
	patches = append(patches, removeLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/nfd")...)
	patches = append(patches, removeLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/node-feature-discovery")...)

	// Patch the node object in the apiserver
	err = helper.PatchNode(cli, node.Name, patches)
	if err != nil {
		stderrLogger.Printf("error while patching node object: %s", err.Error())
		return err
	}

	// patch node status with extended resource changes
	patches = createExtendedResourcePatches(node, extendedResources)
	err = helper.PatchNodeStatus(cli, node.Name, patches)
	if err != nil {
		stderrLogger.Printf("error while patching extended resources: %s", err.Error())
		return err
	}

	return err
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
func createExtendedResourcePatches(n *api.Node, extendedResources ExtendedResources) []apihelper.JsonPatch {
	patches := []apihelper.JsonPatch{}

	// Form a list of namespaced resource names managed by us
	oldResources := stringToNsNames(n.Annotations[extendedResourceAnnotation], LabelNs)

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
	return filepath.Join(nsToAdd, src)
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
