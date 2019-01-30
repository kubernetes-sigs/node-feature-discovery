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
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/ghodss/yaml"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/apimachinery/pkg/util/validation"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/cpu"
	"sigs.k8s.io/node-feature-discovery/source/cpuid"
	"sigs.k8s.io/node-feature-discovery/source/fake"
	"sigs.k8s.io/node-feature-discovery/source/iommu"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
	"sigs.k8s.io/node-feature-discovery/source/local"
	"sigs.k8s.io/node-feature-discovery/source/memory"
	"sigs.k8s.io/node-feature-discovery/source/network"
	"sigs.k8s.io/node-feature-discovery/source/panic_fake"
	"sigs.k8s.io/node-feature-discovery/source/pci"
	"sigs.k8s.io/node-feature-discovery/source/pstate"
	"sigs.k8s.io/node-feature-discovery/source/rdt"
	"sigs.k8s.io/node-feature-discovery/source/storage"
	"sigs.k8s.io/node-feature-discovery/source/system"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-worker"

	// NodeNameEnv is the environment variable that contains this node's name.
	NodeNameEnv = "NODE_NAME"
)

// package loggers
var (
	stdoutLogger = log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger = log.New(os.Stderr, "", log.LstdFlags)
)

// Global config
type NFDConfig struct {
	Sources struct {
		Kernel *kernel.NFDConfig `json:"kernel,omitempty"`
		Pci    *pci.NFDConfig    `json:"pci,omitempty"`
	} `json:"sources,omitempty"`
}

var config = NFDConfig{}

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// Annotations are used for NFD-related node metadata
type Annotations map[string]string

// Command line arguments
type Args struct {
	labelWhiteList     string
	caFile             string
	certFile           string
	keyFile            string
	configFile         string
	noPublish          bool
	options            string
	oneshot            bool
	server             string
	serverNameOverride string
	sleepInterval      time.Duration
	sources            []string
}

func main() {
	// Assert that the version is known
	if version.Get() == "undefined" {
		stderrLogger.Fatalf("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}
	stdoutLogger.Printf("Node Feature Discovery Worker %s", version.Get())

	// Parse command-line arguments.
	args, err := argsParse(nil)
	if err != nil {
		stderrLogger.Fatalf("failed to parse command line: %v", err)
	}

	// Parse config
	err = configParse(args.configFile, args.options)
	if err != nil {
		stderrLogger.Print(err)
	}

	// Configure the parameters for feature discovery.
	enabledSources, labelWhiteList, err := configureParameters(args.sources, args.labelWhiteList)
	if err != nil {
		stderrLogger.Fatalf("error occurred while configuring parameters: %s", err.Error())
	}

	// Connect to NFD server
	dialOpts := []grpc.DialOption{}
	if args.caFile != "" || args.certFile != "" || args.keyFile != "" {
		// Load client cert for client authentication
		cert, err := tls.LoadX509KeyPair(args.certFile, args.keyFile)
		if err != nil {
			stderrLogger.Fatalf("failed to load client certificate: %v", err)
		}
		// Load CA cert for server cert verification
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
			RootCAs:      caPool,
			ServerName:   args.serverNameOverride,
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(args.server, dialOpts...)
	if err != nil {
		stderrLogger.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewLabelerClient(conn)

	for {
		// Get the set of feature labels.
		labels := createFeatureLabels(enabledSources, labelWhiteList)

		// Update the node with the feature labels.
		if !args.noPublish {
			err := advertiseFeatureLabels(client, labels)
			if err != nil {
				stderrLogger.Fatalf("failed to advertise labels: %s", err.Error())
			}
		}

		if args.oneshot {
			break
		}

		if args.sleepInterval > 0 {
			time.Sleep(args.sleepInterval)
		} else {
			conn.Close()
			// Sleep forever
			select {}
		}
	}
}

// argsParse parses the command line arguments passed to the program.
// The argument argv is passed only for testing purposes.
func argsParse(argv []string) (Args, error) {
	args := Args{}
	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish] [--sources=<sources>] [--label-whitelist=<pattern>]
     [--oneshot | --sleep-interval=<seconds>] [--config=<path>]
     [--options=<config>] [--server=<server>] [--server-name-override=<name>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
  %s -h | --help
  %s --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --config=<path>             Config file to use.
                              [Default: /etc/kubernetes/node-feature-discovery/nfd-worker.conf]
  --options=<config>          Specify config options from command line. Config
                              options are specified in the same format as in the
                              config file (i.e. json or yaml). These options
                              will override settings read from the config file.
                              [Default: ]
  --ca-file=<path>            Root certificate for verifying connections
                              [Default: ]
  --cert-file=<path>          Certificate used for authenticating connections
                              [Default: ]
  --key-file=<path>           Private key matching --cert-file
                              [Default: ]
  --server=<server>           NFD server address to connecto to.
                              [Default: localhost:8080]
  --server-name-override=<name> Name (CN) expect from server certificate, useful
                              in testing
                              [Default: ]
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpu,cpuid,iommu,kernel,local,memory,network,pci,pstate,rdt,storage,system]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server. [Default: ]
  --oneshot                   Label once and exit.
  --sleep-interval=<seconds>  Time to sleep between re-labeling. Non-positive
                              value implies no re-labeling (i.e. infinite
                              sleep). [Default: 60s]`,
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
	args.configFile = arguments["--config"].(string)
	args.keyFile = arguments["--key-file"].(string)
	args.noPublish = arguments["--no-publish"].(bool)
	args.options = arguments["--options"].(string)
	args.server = arguments["--server"].(string)
	args.serverNameOverride = arguments["--server-name-override"].(string)
	args.sources = strings.Split(arguments["--sources"].(string), ",")
	args.labelWhiteList = arguments["--label-whitelist"].(string)
	args.oneshot = arguments["--oneshot"].(bool)
	args.sleepInterval, err = time.ParseDuration(arguments["--sleep-interval"].(string))

	// Check that sleep interval has a sane value
	if err != nil {
		return args, fmt.Errorf("invalid --sleep-interval specified: %s", err.Error())
	}
	if args.sleepInterval > 0 && args.sleepInterval < time.Second {
		stderrLogger.Printf("WARNING: too short sleep-intervall specified (%s), forcing to 1s", args.sleepInterval.String())
		args.sleepInterval = time.Second
	}

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

// Parse configuration options
func configParse(filepath string, overrides string) error {
	config.Sources.Kernel = &kernel.Config
	config.Sources.Pci = &pci.Config

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("Failed to read config file: %s", err)
	}

	// Read config file
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("Failed to parse config file: %s", err)
	}

	// Parse config overrides
	err = yaml.Unmarshal([]byte(overrides), &config)
	if err != nil {
		return fmt.Errorf("Failed to parse --options: %s", err)
	}

	return nil
}

// configureParameters returns all the variables required to perform feature
// discovery based on command line arguments.
func configureParameters(sourcesWhiteList []string, labelWhiteListStr string) (enabledSources []source.FeatureSource, labelWhiteList *regexp.Regexp, err error) {
	// A map for lookup
	sourcesWhiteListMap := map[string]struct{}{}
	for _, s := range sourcesWhiteList {
		sourcesWhiteListMap[strings.TrimSpace(s)] = struct{}{}
	}

	// Configure feature sources.
	allSources := []source.FeatureSource{
		cpu.Source{},
		cpuid.Source{},
		fake.Source{},
		iommu.Source{},
		kernel.Source{},
		memory.Source{},
		network.Source{},
		panic_fake.Source{},
		pci.Source{},
		pstate.Source{},
		rdt.Source{},
		storage.Source{},
		system.Source{},
		// local needs to be the last source so that it is able to override
		// labels from other sources
		local.Source{},
	}

	enabledSources = []source.FeatureSource{}
	for _, s := range allSources {
		if _, enabled := sourcesWhiteListMap[s.Name()]; enabled {
			enabledSources = append(enabledSources, s)
		}
	}

	// Compile labelWhiteList regex
	labelWhiteList, err = regexp.Compile(labelWhiteListStr)
	if err != nil {
		stderrLogger.Printf("error parsing whitelist regex (%s): %s", labelWhiteListStr, err)
		return nil, nil, err
	}

	return enabledSources, labelWhiteList, nil
}

// createFeatureLabels returns the set of feature labels from the enabled
// sources and the whitelist argument.
func createFeatureLabels(sources []source.FeatureSource, labelWhiteList *regexp.Regexp) (labels Labels) {
	labels = Labels{}

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

// getFeatureLabels returns node labels for features discovered by the
// supplied source.
func getFeatureLabels(source source.FeatureSource) (labels Labels, err error) {
	defer func() {
		if r := recover(); r != nil {
			stderrLogger.Printf("panic occurred during discovery of source [%s]: %v", source.Name(), r)
			err = fmt.Errorf("%v", r)
		}
	}()

	labels = Labels{}
	features, err := source.Discover()
	if err != nil {
		return nil, err
	}
	for k, v := range features {
		// Validate label name
		prefix := source.Name() + "-"
		switch source.(type) {
		case local.Source:
			// Do not prefix labels from the hooks
			prefix = ""
		}

		label := prefix + k
		// Validate label name. Use dummy namespace 'ns' because there is no
		// function to validate just the name part
		errs := validation.IsQualifiedName("ns/" + label)
		if len(errs) > 0 {
			stderrLogger.Printf("Ignoring invalid feature name '%s': %s", label, errs)
			continue
		}

		value := fmt.Sprintf("%v", v)
		// Validate label value
		errs = validation.IsValidLabelValue(value)
		if len(errs) > 0 {
			stderrLogger.Printf("Ignoring invalid feature value %s=%s: %s", label, value, errs)
			continue
		}

		labels[label] = value
	}
	return labels, nil
}

// advertiseFeatureLabels advertises the feature labels to a Kubernetes node
// via the NFD server.
func advertiseFeatureLabels(client pb.LabelerClient, labels Labels) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodeName := os.Getenv(NodeNameEnv)
	stdoutLogger.Printf("%s: %s", NodeNameEnv, nodeName)

	labelReq := pb.SetLabelsRequest{Labels: labels,
		NfdVersion: version.Get(),
		NodeName:   nodeName}
	rsp, err := client.SetLabels(ctx, &labelReq)
	if err != nil {
		stderrLogger.Printf("failed to set node labels: %v", err)
		return err
	}
	log.Printf("RESPONSE: %s", rsp)

	return nil
}
