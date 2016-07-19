/*
Copyright 2016 Intel Corporation
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
	"os"
	"strings"

	"github.com/docopt/docopt-go"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	// ProgramName is the canonical name of this discovery program.
	ProgramName = "dbi-ia-feature-discovery"

	// Namespace is the prefix for all published labels.
	Namespace = "node.alpha.intel.com"

	// PodNameEnv is the environment variable that contains this pod's name.
	PodNameEnv = "POD_NAME"

	// PodNamespaceEnv is the environment variable that contains this pod's
	// namespace.
	PodNamespaceEnv = "POD_NAMESPACE"
)

var (
	version = "" // Must not be const, set using ldflags at build time
	prefix  = fmt.Sprintf("%s/%s", Namespace, version)
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

func main() {
	// Assert that the version is known
	if version == "" {
		log.Fatalf("main.version not set! Set -ldflags \"-X main.version `git describe --tags --dirty --always`\" during build or run.")
	}

	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish --sources=<sources>]
  %s -h | --help
  %s --version

  Options:
  -h --help           Show this screen.
  --version           Output version and exit.
  --sources=<sources> Comma separated list of feature sources.
                      [Default: cpuid,rdt,pstate]
  --no-publish        Do not publish discovered features to the cluster-local
                      Kubernetes API server.`,
		ProgramName,
		ProgramName,
		ProgramName,
		ProgramName,
	)

	arguments, _ := docopt.Parse(usage, nil, true,
		fmt.Sprintf("%s %s", ProgramName, version), false)

	// Parse argument values as usable types.
	noPublish := arguments["--no-publish"].(bool)
	sourcesArg := strings.Split(arguments["--sources"].(string), ",")

	enabledSources := map[string]struct{}{}
	for _, s := range sourcesArg {
		enabledSources[strings.TrimSpace(s)] = struct{}{}
	}

	// Configure feature sources.
	allSources := []FeatureSource{
		cpuidSource{},
		rdtSource{},
		pstateSource{},
	}

	sources := []FeatureSource{}
	for _, s := range allSources {
		if _, enabled := enabledSources[s.Name()]; enabled {
			sources = append(sources, s)
		}
	}

	labels := Labels{}

	// Add the version of this discovery code as a node label
	versionLabel := fmt.Sprintf("%s/%s.version", Namespace, ProgramName)
	labels[versionLabel] = version
	// Log version label.
	log.Printf("%s = %s", versionLabel, version)

	// Do feature discovery from all configured sources.
	for _, source := range sources {
		for name, value := range featureLabels(source) {
			labels[name] = value
			// Log discovered feature.
			log.Printf("%s = %s", name, value)
		}
	}

	// Update the node with the node labels, unless disabled via flags.
	if !noPublish {
		// Set up K8S client.
		cli, err := client.NewInCluster()
		if err != nil {
			log.Fatalf("can't get kubernetes client: %s", err.Error())
		}

		// Get the current node.
		node := getNode(cli)
		// Add labels to the node object.
		addLabels(node, labels)
		// Send the updated node to the apiserver.
		_, err = cli.Nodes().Update(node)
		if err != nil {
			log.Fatalf("can't update node: %s", err.Error())
		}
	}
}

// featureLabels returns node labels for features discovered by the
// supplied source.
func featureLabels(source FeatureSource) Labels {
	labels := Labels{}
	features, err := source.Discover()
	if err != nil {
		log.Fatalf("discovery failed for source [%s]: %s", source.Name(), err.Error())
	}
	for _, f := range features {
		labels[fmt.Sprintf("%s-%s-%s", prefix, source.Name(), f)] = "true"
	}
	return labels
}

// addLabels modifies the supplied node's labels collection.
//
// In order to publish the labels, the node must be subsequently updated via the
// API server using the client library.
func addLabels(n *api.Node, labels Labels) {
	for k, v := range labels {
		n.Labels[k] = v
	}
}

// getNode returns the Kubernetes node on which this container is running.
func getNode(cli *client.Client) *api.Node {
	// Get the pod name and pod namespace from the env variables
	podName := os.Getenv(PodNameEnv)
	podns := os.Getenv(PodNamespaceEnv)
	log.Printf("%s: %s", PodNameEnv, podName)
	log.Printf("%s: %s", PodNamespaceEnv, podns)

	// Get the pod object using the pod name and pod namespace
	pod, err := cli.Pods(podns).Get(podName)
	if err != nil {
		log.Fatalf("can't get pod: %s", err.Error())
	}

	// Get the node object using the pod name and pod namespace
	node, err := cli.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		log.Fatalf("can't get node: %s", err.Error())
	}

	return node
}
