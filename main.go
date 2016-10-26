package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/docopt/docopt-go"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	// ProgramName is the canonical name of this discovery program.
	ProgramName = "node-feature-discovery"

	// Namespace is the prefix for all published labels.
	Namespace = "node.alpha.kubernetes-incubator.io"

	// PodNameEnv is the environment variable that contains this pod's name.
	PodNameEnv = "POD_NAME"

	// PodNamespaceEnv is the environment variable that contains this pod's
	// namespace.
	PodNamespaceEnv = "POD_NAMESPACE"
)

var (
	version = "" // Must not be const, set using ldflags at build time
	prefix  = fmt.Sprintf("%s/nfd", Namespace)
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// APIHelpers represents a set of API helpers for Kubernetes
type APIHelpers interface {
	// GetClient returns a client
	GetClient() (*client.Client, error)

	// GetNode returns the Kubernetes node on which this container is running.
	GetNode(*client.Client) (*api.Node, error)

	// addLabels modifies the supplied node's labels collection.
	// In order to publish the labels, the node must be subsequently updated via the
	// API server using the client library.
	AddLabels(*api.Node, Labels)

	// UpdateNode updates the node via the API server using a client.
	UpdateNode(*client.Client, *api.Node) error
}

func main() {
	// Assert that the version is known
	if version == "" {
		log.Fatalf("main.version not set! Set -ldflags \"-X main.version `git describe --tags --dirty --always`\" during build or run.")
	}

	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish --sources=<sources> --label-whitelist=<pattern>]
  %s -h | --help
  %s --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpuid,rdt,pstate]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server. [Default: ]`,
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
	whiteListArg := arguments["--label-whitelist"].(string)

	enabledSources := map[string]struct{}{}
	for _, s := range sourcesArg {
		enabledSources[strings.TrimSpace(s)] = struct{}{}
	}

	// Configure feature sources.
	allSources := []FeatureSource{
		cpuidSource{},
		rdtSource{},
		pstateSource{},
		fakeSource{},
	}

	sources := []FeatureSource{}
	for _, s := range allSources {
		if _, enabled := enabledSources[s.Name()]; enabled {
			sources = append(sources, s)
		}
	}

	// compile whiteListArg regex
	labelWhiteList, err := regexp.Compile(whiteListArg)
	if err != nil {
		log.Fatalf("Error parsing whitelist regex (%s): %s", whiteListArg, err)
	}

	labels := Labels{}
	// Add the version of this discovery code as a node label
	versionLabel := fmt.Sprintf("%s/%s.version", Namespace, ProgramName)
	labels[versionLabel] = version
	// Log version label.
	log.Printf("%s = %s", versionLabel, version)

	// Do feature discovery from all configured sources.
	for _, source := range sources {
		labelsFromSource, err := getFeatureLabels(source)
		if err != nil {
			log.Fatalf("discovery failed for source [%s]: %s", source.Name(), err.Error())
		}

		for name, value := range labelsFromSource {
			// Log discovered feature.
			log.Printf("%s = %s", name, value)
			// Skip if label doesn't match labelWhiteList
			if !labelWhiteList.Match([]byte(name)) {
				log.Printf("%s does not match the whitelist (%s) and will not be published.", name, whiteListArg)
				continue
			}
			labels[name] = value
		}
	}

	// Update the node with the node labels, unless disabled via flags.
	if !noPublish {
		helper := APIHelpers(k8sHelpers{})
		err := advertiseFeatureLabels(helper, labels)
		if err != nil {
			log.Fatalf("failed to advertise labels: %s", err.Error())
		}
	}
}

// getFeatureLabels returns node labels for features discovered by the
// supplied source.
func getFeatureLabels(source FeatureSource) (Labels, error) {
	labels := Labels{}
	features, err := source.Discover()
	if err != nil {
		return nil, err
	}
	for _, f := range features {
		labels[fmt.Sprintf("%s-%s-%s", prefix, source.Name(), f)] = "true"
	}
	return labels, nil
}

// advertiseFeatureLabels advertises the feature labels to a Kubernetes node
// via the API server.
func advertiseFeatureLabels(helper APIHelpers, labels Labels) error {
	// Set up K8S client.
	cli, err := helper.GetClient()
	if err != nil {
		log.Printf("can't get kubernetes client: %s", err.Error())
		return err
	}

	// Get the current node.
	node, err := helper.GetNode(cli)
	if err != nil {
		log.Printf("failed to get node: %s", err.Error())
		return err
	}

	// Add labels to the node object.
	helper.AddLabels(node, labels)

	// Send the updated node to the apiserver.
	err = helper.UpdateNode(cli, node)
	if err != nil {
		log.Printf("can't update node: %s", err.Error())
		return err
	}

	return nil
}

// Implements main.APIHelpers
type k8sHelpers struct{}

func (h k8sHelpers) GetClient() (*client.Client, error) {
	// Set up K8S client.
	cli, err := client.NewInCluster()
	if err != nil {
		return nil, err
	}

	return cli, nil
}

func (h k8sHelpers) GetNode(cli *client.Client) (*api.Node, error) {
	// Get the pod name and pod namespace from the env variables
	podName := os.Getenv(PodNameEnv)
	podns := os.Getenv(PodNamespaceEnv)
	log.Printf("%s: %s", PodNameEnv, podName)
	log.Printf("%s: %s", PodNamespaceEnv, podns)

	// Get the pod object using the pod name and pod namespace
	pod, err := cli.Pods(podns).Get(podName)
	if err != nil {
		log.Printf("can't get pods: %s", err.Error())
		return nil, err
	}

	// Get the node object using the pod name and pod namespace
	node, err := cli.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		log.Printf("can't get node: %s", err.Error())
		return nil, err
	}

	return node, nil
}

func (h k8sHelpers) AddLabels(n *api.Node, labels Labels) {
	for k, v := range labels {
		n.Labels[k] = v
	}
}

func (h k8sHelpers) UpdateNode(c *client.Client, n *api.Node) error {
	// Send the updated node to the apiserver.
	_, err := c.Nodes().Update(n)
	if err != nil {
		return err
	}

	return nil
}
