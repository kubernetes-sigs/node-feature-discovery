package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/docopt/docopt-go"
	k8sclient "k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	restclient "k8s.io/client-go/rest"
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

// package loggers
var (
	stdoutLogger = log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger = log.New(os.Stderr, "", log.LstdFlags)
)

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// APIHelpers represents a set of API helpers for Kubernetes
type APIHelpers interface {
	// GetClient returns a client
	GetClient() (*k8sclient.Clientset, error)

	// GetNode returns the Kubernetes node on which this container is running.
	GetNode(*k8sclient.Clientset) (*api.Node, error)

	// RemoveLabels removes labels from the supplied node that contain the
	// search string provided. In order to publish the changes, the node must
	// subsequently be updated via the API server using the client library.
	RemoveLabels(*api.Node, string)

	// AddLabels modifies the supplied node's labels collection.
	// In order to publish the labels, the node must be subsequently updated via the
	// API server using the client library.
	AddLabels(*api.Node, Labels)

	// UpdateNode updates the node via the API server using a client.
	UpdateNode(*k8sclient.Clientset, *api.Node) error
}

func main() {
	// Assert that the version is known
	if version == "" {
		stderrLogger.Fatalf("main.version not set! Set -ldflags \"-X main.version `git describe --tags --dirty --always`\" during build or run.")
	}

	// Parse command-line arguments.
	noPublish, sourcesArg, whiteListArg := argsParse(nil)

	// Configure the parameters for feature discovery.
	sources, labelWhiteList, err := configureParameters(sourcesArg, whiteListArg)
	if err != nil {
		stderrLogger.Fatalf("error occured while configuring parameters: %s", err.Error())
	}

	// Get the set of feature labels.
	labels := createFeatureLabels(sources, labelWhiteList)

	helper := APIHelpers(k8sHelpers{})
	// Update the node with the feature labels.
	err = updateNodeWithFeatureLabels(helper, noPublish, labels)
	if err != nil {
		stderrLogger.Fatalf("error occured while updating node with feature labels: %s", err.Error())
	}
}

// argsParse parses the command line arguments passed to the program.
// The argument argv is passed only for testing purposes.
func argsParse(argv []string) (noPublish bool, sourcesArg []string, whiteListArg string) {
	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish --sources=<sources> --label-whitelist=<pattern>]
  %s -h | --help
  %s --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpuid,rdt,pstate,network]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server. [Default: ]`,
		ProgramName,
		ProgramName,
		ProgramName,
		ProgramName,
	)

	arguments, _ := docopt.Parse(usage, argv, true,
		fmt.Sprintf("%s %s", ProgramName, version), false)

	// Parse argument values as usable types.
	noPublish = arguments["--no-publish"].(bool)
	sourcesArg = strings.Split(arguments["--sources"].(string), ",")
	whiteListArg = arguments["--label-whitelist"].(string)

	return noPublish, sourcesArg, whiteListArg
}

// configureParameters returns all the variables required to perform feature
// discovery based on command line arguments.
func configureParameters(sourcesArg []string, whiteListArg string) (sources []FeatureSource, labelWhiteList *regexp.Regexp, err error) {
	enabledSources := map[string]struct{}{}
	for _, s := range sourcesArg {
		enabledSources[strings.TrimSpace(s)] = struct{}{}
	}

	// Configure feature sources.
	allSources := []FeatureSource{
		cpuidSource{},
		rdtSource{},
		pstateSource{},
		networkSource{},
		fakeSource{},
	}

	sources = []FeatureSource{}
	for _, s := range allSources {
		if _, enabled := enabledSources[s.Name()]; enabled {
			sources = append(sources, s)
		}
	}

	// Compile whiteListArg regex
	labelWhiteList, err = regexp.Compile(whiteListArg)
	if err != nil {
		stderrLogger.Printf("error parsing whitelist regex (%s): %s", whiteListArg, err)
		return nil, nil, err
	}

	return sources, labelWhiteList, nil
}

// createFeatureLabels returns the set of feature labels from the enabled
// sources and the whitelist argument.
func createFeatureLabels(sources []FeatureSource, labelWhiteList *regexp.Regexp) (labels Labels) {
	labels = Labels{}
	// Add the version of this discovery code as a node label
	versionLabel := fmt.Sprintf("%s/%s.version", Namespace, ProgramName)
	labels[versionLabel] = version

	// Log version label.
	stdoutLogger.Printf("%s = %s", versionLabel, version)

	// Do feature discovery from all configured sources.
	for _, source := range sources {
		labelsFromSource, err := getFeatureLabels(source)
		if err != nil {
			stderrLogger.Printf("discovery failed for source [%s]: %s", source.Name(), err.Error())
			stderrLogger.Printf("continuing ...")
			continue
		}

		for name, value := range labelsFromSource {
			// Log discovered feature.
			stdoutLogger.Printf("%s = %s", name, value)
			// Skip if label doesn't match labelWhiteList
			if !labelWhiteList.Match([]byte(name)) {
				stderrLogger.Printf("%s does not match the whitelist (%s) and will not be published.", name, labelWhiteList.String())
				continue
			}
			labels[name] = value
		}
	}
	return labels
}

// updateNodeWithFeatureLabels updates the node with the feature labels, unless
// disabled via --no-publish flag.
func updateNodeWithFeatureLabels(helper APIHelpers, noPublish bool, labels Labels) error {
	if !noPublish {
		err := advertiseFeatureLabels(helper, labels)
		if err != nil {
			stderrLogger.Printf("failed to advertise labels: %s", err.Error())
			return err
		}
	}
	return nil
}

// getFeatureLabels returns node labels for features discovered by the
// supplied source.
func getFeatureLabels(source FeatureSource) (labels Labels, err error) {
	defer func() {
		if r := recover(); r != nil {
			stderrLogger.Printf("panic occured during discovery of source [%s]: %v", source.Name(), r)
			err = fmt.Errorf("%v", r)
		}
	}()

	labels = Labels{}
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
	cli, err := helper.GetClient()
	if err != nil {
		stderrLogger.Printf("can't get kubernetes client: %s", err.Error())
		return err
	}

	// Get the current node.
	node, err := helper.GetNode(cli)
	if err != nil {
		stderrLogger.Printf("failed to get node: %s", err.Error())
		return err
	}

	// Remove labels with our prefix
	helper.RemoveLabels(node, prefix)
	// Add labels to the node object.
	helper.AddLabels(node, labels)

	// Send the updated node to the apiserver.
	err = helper.UpdateNode(cli, node)
	if err != nil {
		stderrLogger.Printf("can't update node: %s", err.Error())
		return err
	}

	return nil
}

// Implements main.APIHelpers
type k8sHelpers struct{}

func (h k8sHelpers) GetClient() (*k8sclient.Clientset, error) {
	// Set up an in-cluster K8S client.
	config, err := restclient.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := k8sclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func (h k8sHelpers) GetNode(cli *k8sclient.Clientset) (*api.Node, error) {
	// Get the pod name and pod namespace from the env variables
	podName := os.Getenv(PodNameEnv)
	podns := os.Getenv(PodNamespaceEnv)
	stdoutLogger.Printf("%s: %s", PodNameEnv, podName)
	stdoutLogger.Printf("%s: %s", PodNamespaceEnv, podns)

	// Get the pod object using the pod name and pod namespace
	pod, err := cli.Core().Pods(podns).Get(podName)
	if err != nil {
		stderrLogger.Printf("can't get pods: %s", err.Error())
		return nil, err
	}

	// Get the node object using the pod name and pod namespace
	node, err := cli.Core().Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		stderrLogger.Printf("can't get node: %s", err.Error())
		return nil, err
	}

	return node, nil
}

// RemoveLabels searches through all labels on Node n and removes
// any where the key contain the search string.
func (h k8sHelpers) RemoveLabels(n *api.Node, search string) {
	for k := range n.Labels {
		if strings.Contains(k, search) {
			delete(n.Labels, k)
		}
	}
}

func (h k8sHelpers) AddLabels(n *api.Node, labels Labels) {
	for k, v := range labels {
		n.Labels[k] = v
	}
}

func (h k8sHelpers) UpdateNode(c *k8sclient.Clientset, n *api.Node) error {
	// Send the updated node to the apiserver.
	_, err := c.Core().Nodes().Update(n)
	if err != nil {
		return err
	}

	return nil
}
