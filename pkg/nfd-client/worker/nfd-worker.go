/*
Copyright 2019-2021 The Kubernetes Authors.

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

package worker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/nfd-client"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/node-feature-discovery/source"

	// Register all source packages
	_ "sigs.k8s.io/node-feature-discovery/source/cpu"
	_ "sigs.k8s.io/node-feature-discovery/source/custom"
	_ "sigs.k8s.io/node-feature-discovery/source/fake"
	_ "sigs.k8s.io/node-feature-discovery/source/iommu"
	_ "sigs.k8s.io/node-feature-discovery/source/kernel"
	_ "sigs.k8s.io/node-feature-discovery/source/local"
	_ "sigs.k8s.io/node-feature-discovery/source/memory"
	_ "sigs.k8s.io/node-feature-discovery/source/network"
	_ "sigs.k8s.io/node-feature-discovery/source/pci"
	_ "sigs.k8s.io/node-feature-discovery/source/storage"
	_ "sigs.k8s.io/node-feature-discovery/source/system"
	_ "sigs.k8s.io/node-feature-discovery/source/usb"
)

// NFDConfig contains the configuration settings of NfdWorker.
type NFDConfig struct {
	Core    coreConfig
	Sources sourcesConfig
}

type coreConfig struct {
	Klog           map[string]string
	LabelWhiteList utils.RegexpVal
	NoPublish      bool
	FeatureSources []string
	Sources        *[]string
	LabelSources   []string
	SleepInterval  duration
}

type sourcesConfig map[string]source.Config

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// Args are the command line arguments of NfdWorker.
type Args struct {
	nfdclient.Args

	ConfigFile string
	Oneshot    bool
	Options    string

	Klog      map[string]*utils.KlogFlagVal
	Overrides ConfigOverrideArgs
}

// ConfigOverrideArgs are args that override config file options
type ConfigOverrideArgs struct {
	NoPublish *bool

	// Deprecated
	LabelWhiteList *utils.RegexpVal
	SleepInterval  *time.Duration
	FeatureSources *utils.StringSliceVal
	LabelSources   *utils.StringSliceVal
}

type nfdWorker struct {
	nfdclient.NfdBaseClient

	args           Args
	certWatch      *utils.FsWatcher
	client         pb.LabelerClient
	configFilePath string
	config         *NFDConfig
	stop           chan struct{} // channel for signaling stop
	featureSources []source.FeatureSource
	labelSources   []source.LabelSource
}

type duration struct {
	time.Duration
}

// NewNfdWorker creates new NfdWorker instance.
func NewNfdWorker(args *Args) (nfdclient.NfdClient, error) {
	base, err := nfdclient.NewNfdBaseClient(&args.Args)
	if err != nil {
		return nil, err
	}

	nfd := &nfdWorker{
		NfdBaseClient: base,

		args:   *args,
		config: &NFDConfig{},
		stop:   make(chan struct{}, 1),
	}

	if args.ConfigFile != "" {
		nfd.configFilePath = filepath.Clean(args.ConfigFile)
	}

	return nfd, nil
}

func newDefaultConfig() *NFDConfig {
	return &NFDConfig{
		Core: coreConfig{
			LabelWhiteList: utils.RegexpVal{Regexp: *regexp.MustCompile("")},
			SleepInterval:  duration{60 * time.Second},
			FeatureSources: []string{"all"},
			LabelSources:   []string{"all"},
			Klog:           make(map[string]string),
		},
	}
}

// Run NfdWorker client. Returns if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the worker args.
func (w *nfdWorker) Run() error {
	klog.Infof("Node Feature Discovery Worker %s", version.Get())
	klog.Infof("NodeName: '%s'", nfdclient.NodeName())

	// Create watcher for config file and read initial configuration
	configWatch, err := utils.CreateFsWatcher(time.Second, w.configFilePath)
	if err != nil {
		return err
	}
	if err := w.configure(w.configFilePath, w.args.Options); err != nil {
		return err
	}

	// Create watcher for TLS certificates
	w.certWatch, err = utils.CreateFsWatcher(time.Second, w.args.CaFile, w.args.CertFile, w.args.KeyFile)
	if err != nil {
		return err
	}

	// Connect to NFD master
	err = w.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer w.Disconnect()

	labelTrigger := time.After(0)
	for {
		select {
		case <-labelTrigger:
			// Run feature discovery
			for _, s := range w.featureSources {
				klog.V(2).Infof("running discovery for %q source", s.Name())
				if err := s.Discover(); err != nil {
					klog.Errorf("feature discovery of %q source failed: %v", s.Name(), err)
				}
			}

			// Get the set of feature labels.
			labels := createFeatureLabels(w.labelSources, w.config.Core.LabelWhiteList.Regexp)

			// Update the node with the feature labels.
			if w.client != nil {
				err := w.advertiseFeatureLabels(labels)
				if err != nil {
					return fmt.Errorf("failed to advertise labels: %s", err.Error())
				}
			}

			if w.args.Oneshot {
				return nil
			}

			if w.config.Core.SleepInterval.Duration > 0 {
				labelTrigger = time.After(w.config.Core.SleepInterval.Duration)
			}

		case <-configWatch.Events:
			klog.Infof("reloading configuration")
			if err := w.configure(w.configFilePath, w.args.Options); err != nil {
				return err
			}
			// Manage connection to master
			if w.config.Core.NoPublish {
				w.Disconnect()
			} else if w.ClientConn() == nil {
				if err := w.Connect(); err != nil {
					return err
				}
			}
			// Always re-label after a re-config event. This way the new config
			// comes into effect even if the sleep interval is long (or infinite)
			labelTrigger = time.After(0)

		case <-w.certWatch.Events:
			klog.Infof("TLS certificate update, renewing connection to nfd-master")
			w.Disconnect()
			if err := w.Connect(); err != nil {
				return err
			}

		case <-w.stop:
			klog.Infof("shutting down nfd-worker")
			configWatch.Close()
			w.certWatch.Close()
			return nil
		}
	}
}

// Stop NfdWorker
func (w *nfdWorker) Stop() {
	select {
	case w.stop <- struct{}{}:
	default:
	}
}

// Connect creates a client connection to the NFD master
func (w *nfdWorker) Connect() error {
	// Return a dummy connection in case of dry-run
	if w.config.Core.NoPublish {
		return nil
	}

	if err := w.NfdBaseClient.Connect(); err != nil {
		return err
	}

	w.client = pb.NewLabelerClient(w.ClientConn())

	return nil
}

// Disconnect closes the connection to NFD master
func (w *nfdWorker) Disconnect() {
	w.NfdBaseClient.Disconnect()
	w.client = nil
}
func (c *coreConfig) sanitize() {
	if c.SleepInterval.Duration > 0 && c.SleepInterval.Duration < time.Second {
		klog.Warningf("too short sleep-intervall specified (%s), forcing to 1s",
			c.SleepInterval.Duration.String())
		c.SleepInterval = duration{time.Second}
	}
}

func (w *nfdWorker) configureCore(c coreConfig) error {
	// Handle klog
	for k, a := range w.args.Klog {
		if !a.IsSetFromCmdline() {
			v, ok := c.Klog[k]
			if !ok {
				v = a.DefValue()
			}
			if err := a.SetFromConfig(v); err != nil {
				return fmt.Errorf("failed to set logger option klog.%s = %v: %v", k, v, err)
			}
		}
	}
	for k := range c.Klog {
		if _, ok := w.args.Klog[k]; !ok {
			klog.Warningf("unknown logger option in config: %q", k)
		}
	}

	// Determine enabled feature sources
	featureSources := make(map[string]source.FeatureSource)
	for _, name := range c.FeatureSources {
		if name == "all" {
			for n, s := range source.GetAllFeatureSources() {
				if ts, ok := s.(source.SupplementalSource); !ok || !ts.DisableByDefault() {
					featureSources[n] = s
				}
			}
		} else {
			disable := false
			strippedName := name
			if strings.HasPrefix(name, "-") {
				strippedName = name[1:]
				disable = true
			}
			if s := source.GetFeatureSource(strippedName); s != nil {
				if !disable {
					featureSources[name] = s
				} else {
					delete(featureSources, strippedName)
				}
			} else {
				klog.Warningf("skipping unknown feature source %q specified in core.featureSources", name)
			}
		}
	}

	w.featureSources = make([]source.FeatureSource, 0, len(featureSources))
	for _, s := range featureSources {
		w.featureSources = append(w.featureSources, s)
	}

	sort.Slice(w.featureSources, func(i, j int) bool { return w.featureSources[i].Name() < w.featureSources[j].Name() })

	// Determine enabled label sources
	labelSources := make(map[string]source.LabelSource)
	for _, name := range c.LabelSources {
		if name == "all" {
			for n, s := range source.GetAllLabelSources() {
				if ts, ok := s.(source.SupplementalSource); !ok || !ts.DisableByDefault() {
					labelSources[n] = s
				}
			}
		} else {
			disable := false
			strippedName := name
			if strings.HasPrefix(name, "-") {
				strippedName = name[1:]
				disable = true
			}
			if s := source.GetLabelSource(strippedName); s != nil {
				if !disable {
					labelSources[name] = s
				} else {
					delete(labelSources, strippedName)
				}
			} else {
				klog.Warningf("skipping unknown source %q specified in core.sources (or -sources)", name)
			}
		}
	}

	w.labelSources = make([]source.LabelSource, 0, len(labelSources))
	for _, s := range labelSources {
		w.labelSources = append(w.labelSources, s)
	}

	sort.Slice(w.labelSources, func(i, j int) bool {
		iP, jP := w.labelSources[i].Priority(), w.labelSources[j].Priority()
		if iP != jP {
			return iP < jP
		}
		return w.labelSources[i].Name() < w.labelSources[j].Name()
	})

	if klog.V(1).Enabled() {
		n := make([]string, len(w.featureSources))
		for i, s := range w.featureSources {
			n[i] = s.Name()
		}
		klog.Infof("enabled feature sources: %s", strings.Join(n, ", "))

		n = make([]string, len(w.labelSources))
		for i, s := range w.labelSources {
			n[i] = s.Name()
		}
		klog.Infof("enabled label sources: %s", strings.Join(n, ", "))
	}

	return nil
}

// Parse configuration options
func (w *nfdWorker) configure(filepath string, overrides string) error {
	// Create a new default config
	c := newDefaultConfig()
	confSources := source.GetAllConfigurableSources()
	c.Sources = make(map[string]source.Config, len(confSources))
	for _, s := range confSources {
		c.Sources[s.Name()] = s.NewConfig()
	}

	// Try to read and parse config file
	if filepath != "" {
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				klog.Infof("config file %q not found, using defaults", filepath)
			} else {
				return fmt.Errorf("error reading config file: %s", err)
			}
		} else {
			err = yaml.Unmarshal(data, c)
			if err != nil {
				return fmt.Errorf("failed to parse config file: %s", err)
			}

			if c.Core.Sources != nil {
				klog.Warningf("found deprecated 'core.sources' config file option, please use 'core.labelSources' instead")
				c.Core.LabelSources = *c.Core.Sources
			}

			klog.Infof("configuration file %q parsed", filepath)
		}
	}

	// Parse config overrides
	if err := yaml.Unmarshal([]byte(overrides), c); err != nil {
		return fmt.Errorf("failed to parse -options: %s", err)
	}

	if w.args.Overrides.LabelWhiteList != nil {
		c.Core.LabelWhiteList = *w.args.Overrides.LabelWhiteList
	}
	if w.args.Overrides.NoPublish != nil {
		c.Core.NoPublish = *w.args.Overrides.NoPublish
	}
	if w.args.Overrides.SleepInterval != nil {
		c.Core.SleepInterval = duration{*w.args.Overrides.SleepInterval}
	}
	if w.args.Overrides.FeatureSources != nil {
		c.Core.FeatureSources = *w.args.Overrides.FeatureSources
	}
	if w.args.Overrides.LabelSources != nil {
		c.Core.LabelSources = *w.args.Overrides.LabelSources
	}

	c.Core.sanitize()

	w.config = c

	if err := w.configureCore(c.Core); err != nil {
		return err
	}

	// (Re-)configure sources
	for _, s := range confSources {
		s.SetConfig(c.Sources[s.Name()])
	}

	klog.Infof("worker (re-)configuration successfully completed")

	return nil
}

// createFeatureLabels returns the set of feature labels from the enabled
// sources and the whitelist argument.
func createFeatureLabels(sources []source.LabelSource, labelWhiteList regexp.Regexp) (labels Labels) {
	labels = Labels{}

	// Get labels from all enabled label sources
	klog.Info("starting feature discovery...")
	for _, source := range sources {
		labelsFromSource, err := getFeatureLabels(source, labelWhiteList)
		if err != nil {
			klog.Errorf("discovery failed for source %q: %v", source.Name(), err)
			continue
		}

		for name, value := range labelsFromSource {
			labels[name] = value
		}
	}
	klog.Info("feature discovery completed")
	utils.KlogDump(1, "labels discovered by feature sources:", "  ", labels)
	return labels
}

// getFeatureLabels returns node labels for features discovered by the
// supplied source.
func getFeatureLabels(source source.LabelSource, labelWhiteList regexp.Regexp) (labels Labels, err error) {
	labels = Labels{}
	features, err := source.GetLabels()
	if err != nil {
		return nil, err
	}

	// Prefix for labels in the default namespace
	prefix := source.Name() + "-"
	switch source.Name() {
	case "local", "custom":
		// Do not prefix labels from the custom rules, hooks or feature files
		prefix = ""
	}

	for k, v := range features {
		// Split label name into namespace and name compoents. Use dummy 'ns'
		// default namespace because there is no function to validate just
		// the name part
		split := strings.SplitN(k, "/", 2)

		label := prefix + split[0]
		nameForValidation := "ns/" + label
		nameForWhiteListing := label

		if len(split) == 2 {
			label = k
			nameForValidation = label
			nameForWhiteListing = split[1]
		}

		// Validate label name.
		errs := validation.IsQualifiedName(nameForValidation)
		if len(errs) > 0 {
			klog.Warningf("ignoring invalid feature name '%s': %s", label, errs)
			continue
		}

		value := fmt.Sprintf("%v", v)
		// Validate label value
		errs = validation.IsValidLabelValue(value)
		if len(errs) > 0 {
			klog.Warningf("ignoring invalid feature value %s=%s: %s", label, value, errs)
			continue
		}

		// Skip if label doesn't match labelWhiteList
		if !labelWhiteList.MatchString(nameForWhiteListing) {
			klog.Infof("%q does not match the whitelist (%s) and will not be published.", nameForWhiteListing, labelWhiteList.String())
			continue
		}

		labels[label] = value
	}
	return labels, nil
}

// getFeatures returns raw features from all feature sources
func getFeatures() map[string]*feature.DomainFeatures {
	features := make(map[string]*feature.DomainFeatures)

	for name, src := range source.GetAllFeatureSources() {
		features[name] = src.GetFeatures()
	}

	return features
}

// advertiseFeatureLabels advertises the feature labels to a Kubernetes node
// via the NFD server.
func (w *nfdWorker) advertiseFeatureLabels(labels Labels) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	klog.Infof("sending labeling request to nfd-master")

	labelReq := pb.SetLabelsRequest{Labels: labels,
		Features:   getFeatures(),
		NfdVersion: version.Get(),
		NodeName:   nfdclient.NodeName()}
	_, err := w.client.SetLabels(ctx, &labelReq)
	if err != nil {
		klog.Errorf("failed to set node labels: %v", err)
		return err
	}

	return nil
}

// UnmarshalJSON implements the Unmarshaler interface from "encoding/json"
func (d *duration) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case float64:
		d.Duration = time.Duration(val)
	case string:
		var err error
		d.Duration, err = time.ParseDuration(val)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid duration %s", data)
	}
	return nil
}

// UnmarshalJSON implements the Unmarshaler interface from "encoding/json"
func (c *sourcesConfig) UnmarshalJSON(data []byte) error {
	// First do a raw parse to get the per-source data
	raw := map[string]json.RawMessage{}
	err := yaml.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	// Then parse each source-specific data structure
	// NOTE: we expect 'c' to be pre-populated with correct per-source data
	//       types. Non-pre-populated keys are ignored.
	for k, rawv := range raw {
		if v, ok := (*c)[k]; ok {
			err := yaml.Unmarshal(rawv, &v)
			if err != nil {
				return fmt.Errorf("failed to parse %q source config: %v", k, err)
			}
		}
	}

	return nil
}
