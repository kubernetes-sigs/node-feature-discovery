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

package nfdworker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"maps"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	klogutils "sigs.k8s.io/node-feature-discovery/pkg/utils/klog"
	"sigs.k8s.io/yaml"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/node-feature-discovery/source"

	// Register all source packages
	_ "sigs.k8s.io/node-feature-discovery/source/cpu"
	_ "sigs.k8s.io/node-feature-discovery/source/custom"
	_ "sigs.k8s.io/node-feature-discovery/source/fake"
	_ "sigs.k8s.io/node-feature-discovery/source/kernel"
	_ "sigs.k8s.io/node-feature-discovery/source/local"
	_ "sigs.k8s.io/node-feature-discovery/source/memory"
	_ "sigs.k8s.io/node-feature-discovery/source/network"
	_ "sigs.k8s.io/node-feature-discovery/source/pci"
	_ "sigs.k8s.io/node-feature-discovery/source/storage"
	_ "sigs.k8s.io/node-feature-discovery/source/system"
	_ "sigs.k8s.io/node-feature-discovery/source/usb"
)

// NfdWorker is the interface for nfd-worker daemon
type NfdWorker interface {
	Run() error
	Stop()
}

// NFDConfig contains the configuration settings of NfdWorker.
type NFDConfig struct {
	Core    coreConfig
	Sources sourcesConfig
}

type coreConfig struct {
	Klog           klogutils.KlogConfigOpts
	LabelWhiteList utils.RegexpVal
	NoPublish      bool
	NoOwnerRefs    bool
	FeatureSources []string
	Sources        *[]string
	LabelSources   []string
	SleepInterval  utils.DurationVal
}

type sourcesConfig map[string]source.Config

// Labels are a Kubernetes representation of discovered features.
type Labels map[string]string

// Args are the command line arguments of NfdWorker.
type Args struct {
	ConfigFile  string
	Klog        map[string]*utils.KlogFlagVal
	Kubeconfig  string
	Oneshot     bool
	Options     string
	Port        int
	NoOwnerRefs bool

	Overrides ConfigOverrideArgs
}

// ConfigOverrideArgs are args that override config file options
type ConfigOverrideArgs struct {
	NoPublish      *bool
	NoOwnerRefs    *bool
	FeatureSources *utils.StringSliceVal
	LabelSources   *utils.StringSliceVal
}

type nfdWorker struct {
	args                Args
	configFilePath      string
	config              *NFDConfig
	kubernetesNamespace string
	k8sClient           k8sclient.Interface
	nfdClient           nfdclient.Interface
	stop                chan struct{} // channel for signaling stop
	featureSources      []source.FeatureSource
	labelSources        []source.LabelSource
	ownerReference      []metav1.OwnerReference
}

// This ticker can represent infinite and normal intervals.
type infiniteTicker struct {
	*time.Ticker
}

// NfdWorkerOption sets properties of the NfdWorker instance.
type NfdWorkerOption interface {
	apply(*nfdWorker)
}

// WithArgs is used for passing settings from command line arguments.
func WithArgs(args *Args) NfdWorkerOption {
	return &nfdWorkerOpt{f: func(n *nfdWorker) { n.args = *args }}
}

// WithKuberneteClient forces to use the given kubernetes client, without
// initializing one from kubeconfig.
func WithKubernetesClient(cli k8sclient.Interface) NfdWorkerOption {
	return &nfdWorkerOpt{f: func(n *nfdWorker) { n.k8sClient = cli }}
}

// WithNFDClient forces to use the given client for the NFD API, without
// initializing one from kubeconfig.
func WithNFDClient(cli nfdclient.Interface) NfdWorkerOption {
	return &nfdWorkerOpt{f: func(n *nfdWorker) { n.nfdClient = cli }}
}

type nfdWorkerOpt struct {
	f func(*nfdWorker)
}

func (f *nfdWorkerOpt) apply(n *nfdWorker) {
	f.f(n)
}

// NewNfdWorker creates new NfdWorker instance.
func NewNfdWorker(opts ...NfdWorkerOption) (NfdWorker, error) {
	nfd := &nfdWorker{
		config:              &NFDConfig{},
		kubernetesNamespace: utils.GetKubernetesNamespace(),
		stop:                make(chan struct{}),
	}

	for _, o := range opts {
		o.apply(nfd)
	}

	if nfd.args.ConfigFile != "" {
		nfd.configFilePath = filepath.Clean(nfd.args.ConfigFile)
	}

	// k8sClient might've been set via opts by tests
	if nfd.k8sClient == nil {
		kubeconfig, err := utils.GetKubeconfig(nfd.args.Kubeconfig)
		if err != nil {
			return nfd, err
		}
		cli, err := k8sclient.NewForConfig(kubeconfig)
		if err != nil {
			return nfd, err
		}
		nfd.k8sClient = cli
	}

	return nfd, nil
}

func newDefaultConfig() *NFDConfig {
	return &NFDConfig{
		Core: coreConfig{
			LabelWhiteList: utils.RegexpVal{Regexp: *regexp.MustCompile("")},
			SleepInterval:  utils.DurationVal{Duration: 60 * time.Second},
			FeatureSources: []string{"all"},
			LabelSources:   []string{"all"},
			Klog:           make(map[string]string),
		},
	}
}

func (w *nfdWorker) Healthz(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func (i *infiniteTicker) Reset(d time.Duration) {
	switch {
	case d > 0:
		i.Ticker.Reset(d)
	default:
		// If the sleep interval is not a positive number the ticker will act
		// as if it was set to an infinite duration by not ticking.
		i.Stop()
	}
}

// Run feature discovery.
func (w *nfdWorker) runFeatureDiscovery() error {
	discoveryStart := time.Now()
	for _, s := range w.featureSources {
		currentSourceStart := time.Now()
		if err := s.Discover(); err != nil {
			klog.ErrorS(err, "feature discovery failed", "source", s.Name())
		}
		klog.V(3).InfoS("feature discovery completed", "featureSource", s.Name(), "duration", time.Since(currentSourceStart))
	}

	discoveryDuration := time.Since(discoveryStart)
	klog.V(2).InfoS("feature discovery of all sources completed", "duration", discoveryDuration)
	featureDiscoveryDuration.WithLabelValues(utils.NodeName()).Observe(discoveryDuration.Seconds())
	if w.config.Core.SleepInterval.Duration > 0 && discoveryDuration > w.config.Core.SleepInterval.Duration/2 {
		klog.InfoS("feature discovery sources took over half of sleep interval ", "duration", discoveryDuration, "sleepInterval", w.config.Core.SleepInterval.Duration)
	}
	// Get the set of feature labels.
	labels := createFeatureLabels(w.labelSources, w.config.Core.LabelWhiteList.Regexp)

	// Update the node with the feature labels.
	if !w.config.Core.NoPublish {
		return w.advertiseFeatures(labels)
	}

	return nil
}

// Set owner ref
func (w *nfdWorker) setOwnerReference() error {
	ownerReference := []metav1.OwnerReference{}

	if !w.config.Core.NoOwnerRefs {
		// Get pod owner reference
		podName := os.Getenv("POD_NAME")
		// Add pod owner reference if it exists
		if podName != "" {
			if selfPod, err := w.k8sClient.CoreV1().Pods(w.kubernetesNamespace).Get(context.TODO(), podName, metav1.GetOptions{}); err != nil {
				klog.ErrorS(err, "failed to get self pod, cannot inherit ownerReference for NodeFeature")
				return err
			} else {
				for _, owner := range selfPod.OwnerReferences {
					owner.BlockOwnerDeletion = ptr.To(false)
					ownerReference = append(ownerReference, owner)
				}
			}

			podUID := os.Getenv("POD_UID")
			if podUID != "" {
				ownerReference = append(ownerReference, metav1.OwnerReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       podName,
					UID:        types.UID(podUID),
				})
			} else {
				klog.InfoS("Cannot append POD ownerReference to NodeFeature, POD_UID not specified")
			}
		} else {
			klog.InfoS("Cannot set NodeFeature owner references, POD_NAME not specified")
		}
	}

	w.ownerReference = ownerReference

	return nil
}

// Run NfdWorker client. Returns an error if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the worker args.
func (w *nfdWorker) Run() error {
	klog.InfoS("Node Feature Discovery Worker", "version", version.Get(), "nodeName", utils.NodeName(), "namespace", w.kubernetesNamespace)

	// Read configuration file
	err := w.configure(w.configFilePath, w.args.Options)
	if err != nil {
		return err
	}

	// Create ticker for feature discovery and run feature discovery once before the loop.
	labelTrigger := infiniteTicker{Ticker: time.NewTicker(1)}
	labelTrigger.Reset(w.config.Core.SleepInterval.Duration)
	defer labelTrigger.Stop()

	httpMux := http.NewServeMux()

	// Register to metrics server
	promRegistry := prometheus.NewRegistry()
	promRegistry.MustRegister(buildInfo, featureDiscoveryDuration)
	httpMux.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}))
	registerVersion(version.Get())

	err = w.runFeatureDiscovery()
	if err != nil {
		return err
	}

	// Only run feature disovery once if Oneshot is set to 'true'.
	if w.args.Oneshot {
		return nil
	}

	// Register health endpoint (at this point we're "ready and live")
	httpMux.HandleFunc("/healthz", w.Healthz)

	// Start HTTP server
	httpServer := http.Server{Addr: fmt.Sprintf(":%d", w.args.Port), Handler: httpMux}
	go func() {
		klog.InfoS("http server starting", "port", httpServer.Addr)
		klog.InfoS("http server stopped", "exitCode", httpServer.ListenAndServe())
	}()
	defer httpServer.Close() // nolint: errcheck

	for {
		select {
		case <-labelTrigger.C:
			err = w.runFeatureDiscovery()
			if err != nil {
				return err
			}

		case <-w.stop:
			klog.InfoS("shutting down nfd-worker")
			return nil
		}
	}
}

// Stop NfdWorker
func (w *nfdWorker) Stop() {
	close(w.stop)
}

func (c *coreConfig) sanitize() {
	if c.SleepInterval.Duration > 0 && c.SleepInterval.Duration < time.Second {
		klog.InfoS("too short sleep interval specified, forcing to 1s",
			"sleepInterval", c.SleepInterval.String())
		c.SleepInterval = utils.DurationVal{Duration: time.Second}
	}
}

func (w *nfdWorker) configureCore(c coreConfig) error {
	// Handle klog
	err := klogutils.MergeKlogConfiguration(w.args.Klog, c.Klog)
	if err != nil {
		return err
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
				klog.InfoS("skipping unknown source specified in core.featureSources", "featureSource", name)
			}
		}
	}

	w.featureSources = slices.Collect(maps.Values(featureSources))

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
				klog.InfoS("skipping unknown source specified in core.labelSources (or -label-sources)", "labelSource", name)
			}
		}
	}

	w.labelSources = slices.Collect(maps.Values(labelSources))

	sort.Slice(w.labelSources, func(i, j int) bool {
		iP, jP := w.labelSources[i].Priority(), w.labelSources[j].Priority()
		if iP != jP {
			return iP < jP
		}
		return w.labelSources[i].Name() < w.labelSources[j].Name()
	})

	if klogV := klog.V(1); klogV.Enabled() {
		n := make([]string, len(w.featureSources))
		for i, s := range w.featureSources {
			n[i] = s.Name()
		}
		klogV.InfoS("enabled feature sources", "featureSources", n)

		n = make([]string, len(w.labelSources))
		for i, s := range w.labelSources {
			n[i] = s.Name()
		}
		klogV.InfoS("enabled label sources", "labelSources", n)
	}

	err = w.setOwnerReference()
	if err != nil {
		return err
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
		data, err := os.ReadFile(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				klog.InfoS("config file not found, using defaults", "path", filepath)
			} else {
				return fmt.Errorf("error reading config file: %s", err)
			}
		} else {
			err = yaml.Unmarshal(data, c)
			if err != nil {
				return fmt.Errorf("failed to parse config file: %s", err)
			}

			if c.Core.Sources != nil {
				klog.InfoS("usage of deprecated 'core.sources' config file option, please use 'core.labelSources' instead")
				c.Core.LabelSources = *c.Core.Sources
			}

			klog.InfoS("configuration file parsed", "path", filepath)
		}
	}

	// Parse config overrides
	if err := yaml.Unmarshal([]byte(overrides), c); err != nil {
		return fmt.Errorf("failed to parse -options: %s", err)
	}

	if w.args.Overrides.NoPublish != nil {
		c.Core.NoPublish = *w.args.Overrides.NoPublish
	}
	if w.args.Overrides.NoOwnerRefs != nil {
		c.Core.NoOwnerRefs = *w.args.Overrides.NoOwnerRefs
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

	klog.InfoS("configuration successfully updated", "configuration", w.config)
	return nil
}

// createFeatureLabels returns the set of feature labels from the enabled
// sources and the whitelist argument.
func createFeatureLabels(sources []source.LabelSource, labelWhiteList regexp.Regexp) (labels Labels) {
	labels = Labels{}

	// Get labels from all enabled label sources
	klog.InfoS("starting feature discovery...")
	for _, source := range sources {
		labelsFromSource, err := GetFeatureLabels(source, labelWhiteList)
		if err != nil {
			klog.ErrorS(err, "discovery failed", "source", source.Name())
			continue
		}

		maps.Copy(labels, labelsFromSource)
	}
	if klogV := klog.V(1); klogV.Enabled() {
		klogV.InfoS("feature discovery completed", "labels", utils.DelayedDumper(labels))
	} else {
		klog.InfoS("feature discovery completed")
	}
	return labels
}

// getFeatureLabels returns node labels for features discovered by the
// supplied source.
func GetFeatureLabels(source source.LabelSource, labelWhiteList regexp.Regexp) (labels Labels, err error) {
	labels = Labels{}
	features, err := source.GetLabels()
	if err != nil {
		return nil, err
	}

	for k, v := range features {
		name := k
		switch sourceName := source.Name(); sourceName {
		case "local", "custom":
			// No mangling of labels from the custom rules or feature files
		default:
			// Prefix for labels from other sources
			if !strings.Contains(name, "/") {
				name = nfdv1alpha1.FeatureLabelNs + "/" + sourceName + "-" + name
			}
		}
		// Split label name into namespace and name compoents
		split := strings.SplitN(name, "/", 2)

		nameForWhiteListing := name
		if len(split) == 2 {
			nameForWhiteListing = split[1]
		}

		// Validate label name.
		errs := validation.IsQualifiedName(name)
		if len(errs) > 0 {
			klog.InfoS("ignoring label with invalid name", "labelKey", name, "errors", errs)
			continue
		}

		value := fmt.Sprintf("%v", v)
		// Validate label value
		errs = validation.IsValidLabelValue(value)
		if len(errs) > 0 {
			klog.InfoS("ignoring label with invalid value", "labelKey", name, "labelValue", value, "errors", errs)
			continue
		}

		// Skip if label doesn't match labelWhiteList
		if !labelWhiteList.MatchString(nameForWhiteListing) {
			klog.InfoS("label does not match the whitelist and will not be published.", "labelKey", nameForWhiteListing, "regexp", labelWhiteList.String())
			continue
		}

		labels[name] = value
	}
	return labels, nil
}

// advertiseFeatures advertises the features of a Kubernetes node
func (w *nfdWorker) advertiseFeatures(labels Labels) error {
	// Create/update NodeFeature CR object
	if err := w.updateNodeFeatureObject(labels); err != nil {
		return fmt.Errorf("failed to advertise features (via CRD API): %w", err)
	}

	return nil
}

// updateNodeFeatureObject creates/updates the node-specific NodeFeature custom resource.
func (m *nfdWorker) updateNodeFeatureObject(labels Labels) error {
	cli, err := m.getNfdClient()
	if err != nil {
		return err
	}
	nodename := utils.NodeName()
	namespace := m.kubernetesNamespace

	features := source.GetAllFeatures()

	// TODO: we could implement some simple caching of the object, only get it
	// every 10 minutes or so because nobody else should really be modifying it
	if nfr, err := cli.NfdV1alpha1().NodeFeatures(namespace).Get(context.TODO(), nodename, metav1.GetOptions{}); errors.IsNotFound(err) {
		nfr = &nfdv1alpha1.NodeFeature{
			ObjectMeta: metav1.ObjectMeta{
				Name:            nodename,
				Annotations:     map[string]string{nfdv1alpha1.WorkerVersionAnnotation: version.Get()},
				Labels:          map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename},
				OwnerReferences: m.ownerReference,
			},
			Spec: nfdv1alpha1.NodeFeatureSpec{
				Features: *features,
				Labels:   labels,
			},
		}
		klog.InfoS("creating NodeFeature object", "nodefeature", klog.KObj(nfr))

		nfrCreated, err := cli.NfdV1alpha1().NodeFeatures(namespace).Create(context.TODO(), nfr, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create NodeFeature object %q: %w", nfr.Name, err)
		}

		klog.V(4).InfoS("NodeFeature object created", "nodeFeature", utils.DelayedDumper(nfrCreated))
	} else if err != nil {
		return fmt.Errorf("failed to get NodeFeature object: %w", err)
	} else {
		nfrUpdated := nfr.DeepCopy()
		nfrUpdated.Annotations = map[string]string{nfdv1alpha1.WorkerVersionAnnotation: version.Get()}
		nfrUpdated.Labels = map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}
		nfrUpdated.OwnerReferences = m.ownerReference
		nfrUpdated.Spec = nfdv1alpha1.NodeFeatureSpec{
			Features: *features,
			Labels:   labels,
		}

		if !apiequality.Semantic.DeepEqual(nfr, nfrUpdated) {
			klog.InfoS("updating NodeFeature object", "nodefeature", klog.KObj(nfr))
			nfrUpdated, err = cli.NfdV1alpha1().NodeFeatures(namespace).Update(context.TODO(), nfrUpdated, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update NodeFeature object %q: %w", nfr.Name, err)
			}
			klog.V(4).InfoS("NodeFeature object updated", "nodeFeature", utils.DelayedDumper(nfrUpdated))
		} else {
			klog.V(1).InfoS("no changes in NodeFeature object, not updating", "nodefeature", klog.KObj(nfr))
		}
	}
	return nil
}

// getNfdClient returns the clientset for using the nfd CRD api
func (m *nfdWorker) getNfdClient() (nfdclient.Interface, error) {
	if m.nfdClient != nil {
		return m.nfdClient, nil
	}

	kubeconfig, err := utils.GetKubeconfig(m.args.Kubeconfig)
	if err != nil {
		return nil, err
	}

	c, err := nfdclient.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	m.nfdClient = c
	return c, nil
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
