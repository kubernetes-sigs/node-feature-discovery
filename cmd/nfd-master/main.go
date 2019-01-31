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

package main

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

	"github.com/docopt/docopt-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-master"

	// NodeNameEnv is the environment variable that contains this node's name.
	NodeNameEnv = "NODE_NAME"

	// Namespace for feature labels
	labelNs = "feature.node.kubernetes.io/"

	// Namespace for all NFD-related annotations
	annotationNs = "nfd.node.kubernetes.io/"
)

// package loggers
var (
	stdoutLogger = log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger = log.New(os.Stderr, "", log.LstdFlags)
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// Annotations are used for NFD-related node metadata
type Annotations map[string]string

// Command line arguments
type Args struct {
	caFile         string
	certFile       string
	keyFile        string
	labelWhiteList *regexp.Regexp
	noPublish      bool
	port           int
	verifyNodeName bool
}

func main() {
	// Assert that the version is known
	if version.Get() == "undefined" {
		stderrLogger.Fatalf("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}
	stdoutLogger.Printf("Node Feature Discovery Master %s", version.Get())

	// Parse command-line arguments.
	args, err := argsParse(nil)
	if err != nil {
		stderrLogger.Fatalf("failed to parse command line: %v", err)
	}

	helper := apihelper.APIHelpers(apihelper.K8sHelpers{AnnotationNs: annotationNs,
		LabelNs: labelNs})

	if !args.noPublish {
		err := updateMasterNode(helper)
		if err != nil {
			stderrLogger.Fatalf("failed to update master node: %v", err)
		}
	}

	// Create server listening for TCP connections
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", args.port))
	if err != nil {
		stderrLogger.Fatalf("failed to listen: %v", err)
	}

	serverOpts := []grpc.ServerOption{}
	// Enable mutual TLS authentication if --cert-file, --key-file or --ca-file
	// is defined
	if args.certFile != "" || args.keyFile != "" || args.caFile != "" {
		// Load cert for authenticating this server
		cert, err := tls.LoadX509KeyPair(args.certFile, args.keyFile)
		if err != nil {
			stderrLogger.Fatalf("failed to load server certificate: %v", err)
		}
		// Load CA cert for client cert verification
		caCert, err := ioutil.ReadFile(args.caFile)
		if err != nil {
			stderrLogger.Fatalf("failed to read root certificate file: %v", err)
		}
		caPool := x509.NewCertPool()
		if ok := caPool.AppendCertsFromPEM(caCert); !ok {
			stderrLogger.Fatalf("failed to add certificate from '%s'", args.caFile)
		}
		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterLabelerServer(grpcServer, &labelerServer{args: args, apiHelper: helper})
	stdoutLogger.Printf("gRPC server serving on port: %d", args.port)
	grpcServer.Serve(lis)
}

// argsParse parses the command line arguments passed to the program.
// The argument argv is passed only for testing purposes.
func argsParse(argv []string) (Args, error) {
	args := Args{}
	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish] [--label-whitelist=<pattern>] [--port=<port>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
     [--verify-node-name]
  %s -h | --help
  %s --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --port=<port>               Port on which to listen for connections.
                              [Default: 8080]
  --ca-file=<path>            Root certificate for verifying connections
                              [Default: ]
  --cert-file=<path>          Certificate used for authenticating connections
                              [Default: ]
  --key-file=<path>           Private key matching --cert-file
                              [Default: ]
  --verify-node-name		  Verify worker node name against CN from the TLS
                              certificate. Only has effect when TLS authentication
                              has been enabled.
  --no-publish                Do not publish feature labels
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server. [Default: ]`,
		ProgramName,
		ProgramName,
		ProgramName,
		ProgramName,
	)

	arguments, _ := docopt.Parse(usage, argv, true,
		fmt.Sprintf("%s %s", ProgramName, version.Get()), false)

	// Parse argument values as usable types.
	var err error
	args.caFile = arguments["--ca-file"].(string)
	args.certFile = arguments["--cert-file"].(string)
	args.keyFile = arguments["--key-file"].(string)
	args.noPublish = arguments["--no-publish"].(bool)
	args.port, err = strconv.Atoi(arguments["--port"].(string))
	if err != nil {
		return args, fmt.Errorf("invalid --port defined: %s", err)
	}
	args.labelWhiteList, err = regexp.Compile(arguments["--label-whitelist"].(string))
	if err != nil {
		return args, fmt.Errorf("error parsing whitelist regex (%s): %s", arguments["--label-whitelist"], err)
	}
	args.verifyNodeName = arguments["--verify-node-name"].(bool)

	// Check TLS related args
	if args.certFile != "" || args.keyFile != "" || args.caFile != "" {
		if args.certFile == "" {
			return args, fmt.Errorf("--cert-file needs to be specified alongside --key-file and --ca-file")
		}
		if args.keyFile == "" {
			return args, fmt.Errorf("--key-file needs to be specified alongside --cert-file and --ca-file")
		}
		if args.caFile == "" {
			return args, fmt.Errorf("--ca-file needs to be specified alongside --cert-file and --key-file")
		}
	}
	return args, nil
}

// Advertise NFD master information
func updateMasterNode(helper apihelper.APIHelpers) error {
	cli, err := helper.GetClient()
	if err != nil {
		return err
	}
	node, err := helper.GetNode(cli, os.Getenv(NodeNameEnv))
	if err != nil {
		return err
	}

	// Advertise NFD version as an annotation
	helper.AddAnnotations(node, Annotations{"master.version": version.Get()})
	err = helper.UpdateNode(cli, node)
	if err != nil {
		stderrLogger.Printf("can't update node: %s", err.Error())
		return err
	}

	return nil
}

// Filter labels if whitelist has been defined
func filterFeatureLabels(labels *Labels, labelWhiteList *regexp.Regexp) {
	for name := range *labels {
		// Skip if label doesn't match labelWhiteList
		if !labelWhiteList.MatchString(name) {
			stderrLogger.Printf("%s does not match the whitelist (%s) and will not be published.", name, labelWhiteList.String())
			delete(*labels, name)
		}
	}
}

// Implement LabelerServer
type labelerServer struct {
	args      Args
	apiHelper apihelper.APIHelpers
}

// Service SetLabels
func (s *labelerServer) SetLabels(c context.Context, r *pb.SetLabelsRequest) (*pb.SetLabelsReply, error) {
	if s.args.verifyNodeName {
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

	if !s.args.noPublish {
		// Advertise NFD worker version and label names as annotations
		keys := make([]string, 0, len(r.Labels))
		for k, _ := range r.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		annotations := Annotations{"worker.version": r.NfdVersion,
			"feature-labels": strings.Join(keys, ",")}

		err := updateNodeFeatures(s.apiHelper, r.NodeName, r.Labels, annotations)
		if err != nil {
			stderrLogger.Printf("failed to advertise labels: %s", err.Error())
			return &pb.SetLabelsReply{}, err
		}
	}
	return &pb.SetLabelsReply{}, nil
}

// advertiseFeatureLabels advertises the feature labels to a Kubernetes node
// via the API server.
func updateNodeFeatures(helper apihelper.APIHelpers, nodeName string, labels Labels, annotations Annotations) error {
	cli, err := helper.GetClient()
	if err != nil {
		return err
	}

	// Get the worker node object
	node, err := helper.GetNode(cli, nodeName)
	if err != nil {
		return err
	}

	// Remove old labels
	if l, ok := node.Annotations[annotationNs+"feature-labels"]; ok {
		oldLabels := strings.Split(l, ",")
		helper.RemoveLabels(node, oldLabels)
	}

	// Also, remove all labels with the old prefix, and the old version label
	helper.RemoveLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/nfd")
	helper.RemoveLabelsWithPrefix(node, "node.alpha.kubernetes-incubator.io/node-feature-discovery")

	// Add labels to the node object.
	helper.AddLabels(node, labels)

	// Add annotations
	helper.AddAnnotations(node, annotations)

	// Send the updated node to the apiserver.
	err = helper.UpdateNode(cli, node)
	if err != nil {
		stderrLogger.Printf("can't update node: %s", err.Error())
		return err
	}

	return nil
}
