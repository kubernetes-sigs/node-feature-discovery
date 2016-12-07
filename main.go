package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/docopt/docopt-go"
	k8sclient "k8s.io/client-go/kubernetes"
	apitypes "k8s.io/client-go/pkg/api"
	api "k8s.io/client-go/pkg/api/v1"
	restclient "k8s.io/client-go/rest"
)

const (
	// ProgramName is the canonical name of this discovery program.
	ProgramName = "node-feature-discovery"

	// ProgramAbbrev is the abbreviated name of this discovery program.
	ProgramAbbrev = "nfd"

	// Namespace is the prefix for all published labels.
	Namespace = "node.alpha.kubernetes-incubator.io"

	// PodNameEnv is the environment variable that contains this pod's name.
	PodNameEnv = "POD_NAME"

	// PodNamespaceEnv is the environment variable that contains this pod's
	// namespace.
	PodNamespaceEnv = "POD_NAMESPACE"

	// TypeBinary is the class of binary discovered resources.
	TypeBinary = "binary"

	// TypeInteger is the class of integer discovered resources.
	TypeInteger = "integer"
)

var (
	version = "" // Must not be const, set using ldflags at build time
	prefix  = fmt.Sprintf("%s/%s", Namespace, ProgramAbbrev)
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

	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--sources=<sources>] [--resource-types=<rtypes>]
	    [--label-whitelist=<pattern>] [--resource-whitelist=<pattern>]
      [--no-publish]
  %s -h | --help
  %s --version

  Options:
  -h --help                      Show this screen.
  --version                      Output version and exit.
  --sources=<sources>            Comma separated list of feature sources.
                                 [Default: cpuid,rdt,pstate]
  --resource-types=<rtypes>      Comma separated list of resource types to
                                 discover.
                                 [Default: binary,integer]
  --label-whitelist=<pattern>    Regular expression to filter label names to
                                 publish to the Kubernetes API server.
                                 [Default: ]
  --resource-whitelist=<pattern> Regular expression to filter resource names to
                                 publish to the Kubernetes API server.
                                 [Default: ]
  --no-publish                   Do not publish discovered features to the
                                 cluster-local Kubernetes API server.`,
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
	resourceTypesArg := strings.Split(arguments["--resource-types"].(string), ",")
	labelWhiteListArg := arguments["--label-whitelist"].(string)
	resourceWhiteListArg := arguments["--resource-whitelist"].(string)

	enabledSources := map[string]struct{}{}
	for _, s := range sourcesArg {
		enabledSources[strings.TrimSpace(s)] = struct{}{}
	}

	enabledResourceTypes := map[string]struct{}{}
	for _, t := range resourceTypesArg {
		enabledResourceTypes[strings.TrimSpace(t)] = struct{}{}
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

	// Configure resource types.
	allResourceTypes := []string{TypeBinary, TypeInteger}

	resourceTypes := []string{}
	for _, t := range allResourceTypes {
		if _, enabled := enabledResourceTypes[t]; enabled {
			resourceTypes = append(resourceTypes, t)
		}
	}

	// compile white list regexes
	labelWhiteList, err := regexp.Compile(labelWhiteListArg)
	if err != nil {
		stderrLogger.Fatalf("Error parsing label whitelist regex (%s): %s", labelWhiteListArg, err)
	}
	resourceWhiteList, err := regexp.Compile(resourceWhiteListArg)
	if err != nil {
		stderrLogger.Fatalf("Error parsing resource whitelist regex (%s): %s", resourceWhiteListArg, err)
	}

	labels := Labels{}
	resources := api.ResourceList{}

	// Add the version of this discovery code as a node label
	versionLabel := fmt.Sprintf("%s/%s.version", Namespace, ProgramName)
	labels[versionLabel] = version

	// Log version label.
	stdoutLogger.Printf("%s = %s", versionLabel, version)

	// Do feature discovery from all configured sources.
	for _, source := range sources {

		var labelsFromSource Labels
		if _, enabled := enabledResourceTypes[TypeBinary]; enabled {
			labelsFromSource, err = getFeatureLabels(source)
			if err != nil {
				stderrLogger.Printf("label discovery failed for source [%s]: %s", source.Name(), err.Error())
				stderrLogger.Printf("continuing ...")
				continue
			}
		}

		var resourcesFromSource api.ResourceList
		if _, enabled := enabledResourceTypes[TypeInteger]; enabled {
			resourcesFromSource, err = getResources(source)
			if err != nil {
				stderrLogger.Printf("resource discovery failed for source [%s]: %s", source.Name(), err.Error())
				stderrLogger.Printf("continuing ...")
				continue
			}
		}

		for name, value := range labelsFromSource {
			// Log discovered feature.
			stdoutLogger.Printf("%s = %s", name, value)
			// Skip if label doesn't match labelWhiteList
			if !labelWhiteList.Match([]byte(name)) {
				stderrLogger.Printf("%s does not match the label whitelist (%s) and will not be published.", name, labelWhiteListArg)
				continue
			}
			labels[name] = value
		}

		for name, quantity := range resourcesFromSource {
			// Log discovered resource.
			stdoutLogger.Printf("%s = %s", name, quantity.String())
			// Skip if label doesn't match labelWhiteList
			if !resourceWhiteList.Match([]byte(name)) {
				stderrLogger.Printf("%s does not match the resource whitelist (%s) and will not be published.", name, resourceWhiteListArg)
				continue
			}
			resources[name] = quantity
		}
	}

	// Update the node with the node labels, unless disabled via flags.
	if !noPublish {
		helper := APIHelpers(k8sHelpers{})
		err := advertiseFeatureLabels(helper, labels)
		if err != nil {
			stderrLogger.Fatalf("failed to advertise labels: %s", err.Error())
		}

		err = advertiseResources(helper, resources)
		if err != nil {
			stderrLogger.Fatalf("failed to advertise resources: %s", err.Error())
		}
	}
}

// getFeatureLabels returns node labels for features discovered by the
// supplied source.
func getFeatureLabels(source FeatureSource) (labels Labels, err error) {
	defer func() {
		if r := recover(); r != nil {
			stderrLogger.Printf("panic occured during feature discovery of source [%s]: %v", source.Name(), r)
			err = fmt.Errorf("%v", r)
		}
	}()

	labels = Labels{}
	features, err := source.Discover()
	if err != nil {
		return nil, err
	}
	for _, f := range features {
		labels[normalizeLabel(source.Name(), f)] = "true"
	}
	return labels, nil
}

func normalizeLabel(sourceName, feature string) string {
	return fmt.Sprintf("%s-%s-%s", prefix, sourceName, feature)
}

// getResources returns node labels for features discovered by the
// supplied source.
func getResources(source FeatureSource) (result api.ResourceList, err error) {
	defer func() {
		if r := recover(); r != nil {
			stderrLogger.Printf("panic occured during resource discovery of source [%s]: %v", source.Name(), r)
			err = fmt.Errorf("%v", r)
		}
	}()

	resources, err := source.DiscoverResources()
	if err != nil {
		return nil, err
	}
	result = api.ResourceList{}
	for name, quantity := range resources {
		result[normalizeResource(source.Name(), name)] = quantity
	}
	return result, nil
}

// normalizeResource returns a resource name that has the opaque integer
// resource prefix (required for proper handling by the default Kubernetes
// scheduler.) The result resource name also contains the feature source
// that discovered it.
//
// For example, given a raw resource name like "featureX" from feature source
// "source1", the result looks like this:
// `<opaque-int-prefix>-nfd-source1-featureX`
func normalizeResource(sourceName string, resourceName api.ResourceName) api.ResourceName {
	// if the resource name is already has the opaque int resource prefix,
	// strip it before injecting the source name.
	r := string(resourceName)
	if api.IsOpaqueIntResourceName(resourceName) {
		r = strings.TrimPrefix(r, api.ResourceOpaqueIntPrefix)
	}
	return api.OpaqueIntResourceName(fmt.Sprintf("%s-%s-%s", ProgramAbbrev, sourceName, r))
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

// advertiseResources advertises the discovered resources to a Kubernetes node
// via the API server.
func advertiseResources(helper APIHelpers, resources api.ResourceList) error {
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
	patch := []map[string]string{}
	for name, quantity := range resources {
		patch = append(patch, map[string]string{
			"op":    "add",
			"path":  fmt.Sprintf("/status/capacity/%s", escapeForJSONPatch(name)),
			"value": quantity.String(),
		})
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		stderrLogger.Printf("could not prepare patch to advertise opaque resources: %s", err)
	}
	return cli.Core().RESTClient().Patch(apitypes.JSONPatchType).Resource("nodes").Name(node.Name).SubResource("status").Body(patchJSON).Do().Error()
}

func escapeForJSONPatch(resName api.ResourceName) string {
	// Escape forward slashes in the resource name per the JSON Pointer spec.
	// See https://tools.ietf.org/html/rfc6901#section-3
	escaped := string(resName)
	escaped = strings.Replace(escaped, "~", "~0", -1)
	escaped = strings.Replace(escaped, "/", "~1", -1)
	return escaped
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
