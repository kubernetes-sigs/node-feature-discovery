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
	LabelNs = "feature.node.kubernetes.io/"

	// Namespace for all NFD-related annotations
	AnnotationNs = "nfd.node.kubernetes.io/"
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
	ExtraLabelNs   []string
	KeyFile        string
	LabelWhiteList *regexp.Regexp
	NoPublish      bool
	Port           int
	VerifyNodeName bool
	ResourceLabels []string
}

type NfdMaster interface {
	Run() error
	Stop()
	WaitForReady(time.Duration) bool
}

type nfdMaster struct {
	args   Args
	server *grpc.Server
	ready  chan bool
}

// statusOp is a json marshaling helper used for patching node status
type statusOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

func createStatusOp(verb string, resource string, path string, value string) statusOp {
	if !strings.Contains(resource, "/") {
		resource = LabelNs + resource
	}
	res := strings.ReplaceAll(resource, "/", "~1")
	return statusOp{verb, "/status/" + path + "/" + res, value}
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

	return nfd, nil
}

// Run NfdMaster server. The method returns in case of fatal errors or if Stop()
// is called.
func (m *nfdMaster) Run() error {
	stdoutLogger.Printf("Node Feature Discovery Master %s", version.Get())
	stdoutLogger.Printf("NodeName: '%s'", nodeName)

	// Initialize Kubernetes API helpers
	helper := apihelper.APIHelpers(apihelper.K8sHelpers{})

	if !m.args.NoPublish {
		err := updateMasterNode(helper)
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
	pb.RegisterLabelerServer(m.server, &labelerServer{args: m.args, apiHelper: helper})
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
	addAnnotations(node, Annotations{"master.version": version.Get()})
	err = helper.UpdateNode(cli, node)
	if err != nil {
		stderrLogger.Printf("can't update node: %s", err.Error())
		return err
	}

	return nil
}

// Filter labels by namespace and name whitelist
func filterFeatureLabels(labels Labels, extraLabelNs []string, labelWhiteList *regexp.Regexp, extendedResourceNames []string) (Labels, ExtendedResources) {
	for label := range labels {
		split := strings.SplitN(label, "/", 2)
		name := split[0]

		// Check namespaced labels, filter out if ns is not whitelisted
		if len(split) == 2 {
			ns := split[0]
			name = split[1]
			for i, extraNs := range extraLabelNs {
				if ns == extraNs {
					break
				} else if i == len(extraLabelNs)-1 {
					stderrLogger.Printf("Namespace '%s' is not allowed. Ignoring label '%s'\n", ns, label)
					delete(labels, label)
				}
			}
		}

		// Skip if label doesn't match labelWhiteList
		if !labelWhiteList.MatchString(name) {
			stderrLogger.Printf("%s does not match the whitelist (%s) and will not be published.", name, labelWhiteList.String())
			delete(labels, label)
		}
	}

	// Remove labels which are intended to be extended resources
	extendedResources := ExtendedResources{}
	for _, extendedResourceName := range extendedResourceNames {
		// remove possibly given default LabelNs to keep annotations shorter
		extendedResourceName = strings.TrimPrefix(extendedResourceName, LabelNs)
		if _, ok := labels[extendedResourceName]; ok {
			if _, err := strconv.Atoi(labels[extendedResourceName]); err != nil {
				stderrLogger.Printf("bad label value encountered for extended resource: %s", err.Error())
				continue // non-numeric label can't be used
			}

			extendedResources[extendedResourceName] = labels[extendedResourceName]
			delete(labels, extendedResourceName)
		}
	}

	return labels, extendedResources
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
		// Advertise NFD worker version, label names and extended resources as annotations
		labelKeys := make([]string, 0, len(labels))
		for k := range labels {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)

		extendedResourceKeys := make([]string, 0, len(extendedResources))
		for key := range extendedResources {
			extendedResourceKeys = append(extendedResourceKeys, key)
		}
		sort.Strings(extendedResourceKeys)

		annotations := Annotations{"worker.version": r.NfdVersion,
			"feature-labels":     strings.Join(labelKeys, ","),
			"extended-resources": strings.Join(extendedResourceKeys, ","),
		}

		err := updateNodeFeatures(s.apiHelper, r.NodeName, labels, annotations, extendedResources)
		if err != nil {
			stderrLogger.Printf("failed to advertise labels: %s", err.Error())
			return &pb.SetLabelsReply{}, err
		}
	}
	return &pb.SetLabelsReply{}, nil
}

// advertiseFeatureLabels advertises the feature labels to a Kubernetes node
// via the API server.
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

	// Resolve publishable extended resources before node is modified
	statusOps := getExtendedResourceOps(node, extendedResources)

	// Remove old labels
	if l, ok := node.Annotations[AnnotationNs+"feature-labels"]; ok {
		oldLabels := strings.Split(l, ",")
		removeLabels(node, oldLabels)
	}

	// Also, remove all labels with the old prefix, and the old version label
	removeLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/nfd")
	removeLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/node-feature-discovery")

	// Add labels to the node object.
	addLabels(node, labels)

	// Add annotations
	addAnnotations(node, annotations)

	// Send the updated node to the apiserver.
	err = helper.UpdateNode(cli, node)
	if err != nil {
		stderrLogger.Printf("can't update node: %s", err.Error())
		return err
	}

	// patch node status with extended resource changes
	if len(statusOps) > 0 {
		err = helper.PatchStatus(cli, node.Name, statusOps)
		if err != nil {
			stderrLogger.Printf("error while patching extended resources: %s", err.Error())
			return err
		}
	}

	return err
}

// Remove any labels having the given prefix
func removeLabelsWithPrefix(n *api.Node, search string) {
	for k := range n.Labels {
		if strings.HasPrefix(k, search) {
			delete(n.Labels, k)
		}
	}
}

// Removes NFD labels from a Node object
func removeLabels(n *api.Node, labelNames []string) {
	for _, l := range labelNames {
		if strings.Contains(l, "/") {
			delete(n.Labels, l)
		} else {
			delete(n.Labels, LabelNs+l)
		}
	}
}

// getExtendedResourceOps returns a slice of operations to perform on the node status
func getExtendedResourceOps(n *api.Node, extendedResources ExtendedResources) []statusOp {
	var statusOps []statusOp

	oldResources := strings.Split(n.Annotations[AnnotationNs+"extended-resources"], ",")

	// figure out which resources to remove
	for _, resource := range oldResources {
		if _, ok := n.Status.Capacity[api.ResourceName(addNs(resource, LabelNs))]; ok {
			// check if the ext resource is still needed
			_, extResNeeded := extendedResources[resource]
			if !extResNeeded {
				statusOps = append(statusOps, createStatusOp("remove", resource, "capacity", ""))
				statusOps = append(statusOps, createStatusOp("remove", resource, "allocatable", ""))
			}
		}
	}

	// figure out which resources to replace and which to add
	for resource, value := range extendedResources {
		// check if the extended resource already exists with the same capacity in the node
		if quantity, ok := n.Status.Capacity[api.ResourceName(addNs(resource, LabelNs))]; ok {
			val, _ := quantity.AsInt64()
			if strconv.FormatInt(val, 10) != value {
				statusOps = append(statusOps, createStatusOp("replace", resource, "capacity", value))
				statusOps = append(statusOps, createStatusOp("replace", resource, "allocatable", value))
			}
		} else {
			statusOps = append(statusOps, createStatusOp("add", resource, "capacity", value))
			// "allocatable" gets added implicitly after adding to capacity
		}
	}

	return statusOps
}

// Add NFD labels to a Node object.
func addLabels(n *api.Node, labels map[string]string) {
	for k, v := range labels {
		if strings.Contains(k, "/") {
			n.Labels[k] = v
		} else {
			n.Labels[LabelNs+k] = v
		}
	}
}

// Add Annotations to a Node object
func addAnnotations(n *api.Node, annotations map[string]string) {
	for k, v := range annotations {
		n.Annotations[AnnotationNs+k] = v
	}
}

// addNs adds a namespace if one isn't already found from src string
func addNs(src string, nsToAdd string) string {
	if strings.Contains(src, "/") {
		return src
	}
	return nsToAdd + src
}
