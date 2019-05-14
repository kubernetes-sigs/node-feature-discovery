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

package nfdworker

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

	"github.com/ghodss/yaml"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/apimachinery/pkg/util/validation"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/cpu"
	"sigs.k8s.io/node-feature-discovery/source/fake"
	"sigs.k8s.io/node-feature-discovery/source/iommu"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
	"sigs.k8s.io/node-feature-discovery/source/local"
	"sigs.k8s.io/node-feature-discovery/source/memory"
	"sigs.k8s.io/node-feature-discovery/source/network"
	"sigs.k8s.io/node-feature-discovery/source/panic_fake"
	"sigs.k8s.io/node-feature-discovery/source/pci"
	"sigs.k8s.io/node-feature-discovery/source/storage"
	"sigs.k8s.io/node-feature-discovery/source/system"
)

// package loggers
var (
	stdoutLogger = log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger = log.New(os.Stderr, "", log.LstdFlags)
	nodeName     = os.Getenv("NODE_NAME")
)

// Global config
type NFDConfig struct {
	Sources struct {
		Cpu    *cpu.NFDConfig    `json:"cpu,omitempty"`
		Kernel *kernel.NFDConfig `json:"kernel,omitempty"`
		Pci    *pci.NFDConfig    `json:"pci,omitempty"`
	} `json:"sources,omitempty"`
}

var config = NFDConfig{}

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// Command line arguments
type Args struct {
	LabelWhiteList     string
	CaFile             string
	CertFile           string
	KeyFile            string
	ConfigFile         string
	NoPublish          bool
	Options            string
	Oneshot            bool
	Server             string
	ServerNameOverride string
	SleepInterval      time.Duration
	Sources            []string
}

type NfdWorker interface {
	Run() error
}

type nfdWorker struct {
	args Args
}

// Create new NfdWorker instance.
func NewNfdWorker(args Args) (*nfdWorker, error) {
	nfd := &nfdWorker{args: args}
	if args.SleepInterval > 0 && args.SleepInterval < time.Second {
		stderrLogger.Printf("WARNING: too short sleep-intervall specified (%s), forcing to 1s", args.SleepInterval.String())
		args.SleepInterval = time.Second
	}

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

// Run NfdWorker client. Returns if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the worker args.
func (w *nfdWorker) Run() error {
	stdoutLogger.Printf("Node Feature Discovery Worker %s", version.Get())
	stdoutLogger.Printf("NodeName: '%s'", nodeName)

	// Parse config
	err := configParse(w.args.ConfigFile, w.args.Options)
	if err != nil {
		stderrLogger.Print(err)
	}

	// Configure the parameters for feature discovery.
	enabledSources, labelWhiteList, err := configureParameters(w.args.Sources, w.args.LabelWhiteList)
	if err != nil {
		return fmt.Errorf("error occurred while configuring parameters: %s", err.Error())
	}

	// Connect to NFD server
	dialOpts := []grpc.DialOption{}
	if w.args.CaFile != "" || w.args.CertFile != "" || w.args.KeyFile != "" {
		// Load client cert for client authentication
		cert, err := tls.LoadX509KeyPair(w.args.CertFile, w.args.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %v", err)
		}
		// Load CA cert for server cert verification
		caCert, err := ioutil.ReadFile(w.args.CaFile)
		if err != nil {
			return fmt.Errorf("failed to read root certificate file: %v", err)
		}
		caPool := x509.NewCertPool()
		if ok := caPool.AppendCertsFromPEM(caCert); !ok {
			return fmt.Errorf("failed to add certificate from '%s'", w.args.CaFile)
		}
		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
			ServerName:   w.args.ServerNameOverride,
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(w.args.Server, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewLabelerClient(conn)

	for {
		// Get the set of feature labels.
		labels := createFeatureLabels(enabledSources, labelWhiteList)

		// Update the node with the feature labels.
		if !w.args.NoPublish {
			err := advertiseFeatureLabels(client, labels)
			if err != nil {
				return fmt.Errorf("failed to advertise labels: %s", err.Error())
			}
		}

		if w.args.Oneshot {
			break
		}

		if w.args.SleepInterval > 0 {
			time.Sleep(w.args.SleepInterval)
		} else {
			conn.Close()
			// Sleep forever
			select {}
		}
	}
	return nil
}

// Parse configuration options
func configParse(filepath string, overrides string) error {
	config.Sources.Cpu = &cpu.Config
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
		fake.Source{},
		iommu.Source{},
		kernel.Source{},
		memory.Source{},
		network.Source{},
		panic_fake.Source{},
		pci.Source{},
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
		labelName := "ns/" + label
		// Do not use dummy namespace if there is already a namespace
		if strings.Contains(label, "/") {
			labelName = label
		}
		errs := validation.IsQualifiedName(labelName)
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

	stdoutLogger.Printf("Sendng labeling request nfd-master")

	labelReq := pb.SetLabelsRequest{Labels: labels,
		NfdVersion: version.Get(),
		NodeName:   nodeName}
	_, err := client.SetLabels(ctx, &labelReq)
	if err != nil {
		stderrLogger.Printf("failed to set node labels: %v", err)
		return err
	}

	return nil
}
