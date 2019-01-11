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
	"fmt"
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
	labelWhiteList *regexp.Regexp
	noPublish      bool
	port           int
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
	grpcServer := grpc.NewServer()
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
  %s -h | --help
  %s --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --port=<port>               Port on which to listen for connections.
                              [Default: 8080]
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
	args.noPublish = arguments["--no-publish"].(bool)
	args.port, err = strconv.Atoi(arguments["--port"].(string))
	if err != nil {
		return args, fmt.Errorf("invalid --port defined: %s", err)
	}
	args.labelWhiteList, err = regexp.Compile(arguments["--label-whitelist"].(string))
	if err != nil {
		return args, fmt.Errorf("error parsing whitelist regex (%s): %s", arguments["--label-whitelist"], err)
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
