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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
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
	"sigs.k8s.io/node-feature-discovery/pkg/features"
	pb "sigs.k8s.io/node-feature-discovery/pkg/labeler"
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
	CaFile               string
	CertFile             string
	ConfigFile           string
	EnableNodeFeatureApi bool
	KeyFile              string
	Klog                 map[string]*utils.KlogFlagVal
	Kubeconfig           string
	Oneshot              bool
	Options              string
	Server               string
	ServerNameOverride   string
	MetricsPort          int
	GrpcHealthPort       int

	Overrides ConfigOverrideArgs
}

// ConfigOverrideArgs are args that override config file options
type ConfigOverrideArgs struct {
	NoPublish *bool

	FeatureSources *utils.StringSliceVal
	LabelSources   *utils.StringSliceVal
}

type nfdWorker struct {
	args                Args
	certWatch           *utils.FsWatcher
	clientConn          *grpc.ClientConn
	configFilePath      string
	config              *NFDConfig
	kubernetesNamespace string
	grpcClient          pb.LabelerClient
	healthServer        *grpc.Server
	k8sClient           k8sclient.Interface
	nfdClient           *nfdclient.Clientset
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
	return &nfdMWorkerOpt{f: func(n *nfdWorker) { n.args = *args }}
}

// WithKuberneteClient forces to use the given kubernetes client, without
// initializing one from kubeconfig.
func WithKubernetesClient(cli k8sclient.Interface) NfdWorkerOption {
	return &nfdMWorkerOpt{f: func(n *nfdWorker) { n.k8sClient = cli }}
}

type nfdMWorkerOpt struct {
	f func(*nfdWorker)
}

func (f *nfdMWorkerOpt) apply(n *nfdWorker) {
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

	// Check TLS related args
	if nfd.args.CertFile != "" || nfd.args.KeyFile != "" || nfd.args.CaFile != "" {
		if nfd.args.CertFile == "" {
			return nfd, fmt.Errorf("-cert-file needs to be specified alongside -key-file and -ca-file")
		}
		if nfd.args.KeyFile == "" {
			return nfd, fmt.Errorf("-key-file needs to be specified alongside -cert-file and -ca-file")
		}
		if nfd.args.CaFile == "" {
			return nfd, fmt.Errorf("-ca-file needs to be specified alongside -cert-file and -key-file")
		}
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

func (i *infiniteTicker) Reset(d time.Duration) {
	switch {
	case d > 0:
		i.Ticker.Reset(d)
	default:
		// If the sleep interval is not a positive number the ticker will act
		// as if it was set to an infinite duration by not ticking.
		i.Ticker.Stop()
	}
}

func (w *nfdWorker) startGrpcHealthServer(errChan chan<- error) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", w.args.GrpcHealthPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(s, health.NewServer())
	klog.InfoS("gRPC health server serving", "port", w.args.GrpcHealthPort)

	go func() {
		defer func() {
			lis.Close()
		}()
		if err := s.Serve(lis); err != nil {
			errChan <- fmt.Errorf("gRPC health server exited with an error: %w", err)
		}
		klog.InfoS("gRPC health server stopped")
	}()
	w.healthServer = s
	return nil
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

// Run NfdWorker client. Returns if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the worker args.
func (w *nfdWorker) Run() error {
	klog.InfoS("Node Feature Discovery Worker", "version", version.Get(), "nodeName", utils.NodeName(), "namespace", w.kubernetesNamespace)

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

	defer w.grpcDisconnect()

	// Create ticker for feature discovery and run feature discovery once before the loop.
	labelTrigger := infiniteTicker{Ticker: time.NewTicker(1)}
	labelTrigger.Reset(w.config.Core.SleepInterval.Duration)
	defer labelTrigger.Stop()

	// Create owner ref
	ownerReference := []metav1.OwnerReference{}
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

	w.ownerReference = ownerReference

	// Register to metrics server
	if w.args.MetricsPort > 0 {
		m := utils.CreateMetricsServer(w.args.MetricsPort,
			buildInfo,
			featureDiscoveryDuration)
		go m.Run()
		registerVersion(version.Get())
		defer m.Stop()
	}

	err = w.runFeatureDiscovery()
	if err != nil {
		return err
	}

	// Only run feature disovery once if Oneshot is set to 'true'.
	if w.args.Oneshot {
		return nil
	}

	grpcErr := make(chan error)

	// Start gRPC server for liveness probe (at this point we're "live")
	if w.args.GrpcHealthPort != 0 {
		if err := w.startGrpcHealthServer(grpcErr); err != nil {
			return fmt.Errorf("failed to start gRPC health server: %w", err)
		}
	}

	for {
		select {
		case err := <-grpcErr:
			return fmt.Errorf("error in serving gRPC: %w", err)

		case <-labelTrigger.C:
			err = w.runFeatureDiscovery()
			if err != nil {
				return err
			}

		case <-configWatch.Events:
			klog.InfoS("reloading configuration")
			if err := w.configure(w.configFilePath, w.args.Options); err != nil {
				return err
			}
			// Manage connection to master
			if w.config.Core.NoPublish || !features.NFDFeatureGate.Enabled(features.NodeFeatureAPI) || !w.args.EnableNodeFeatureApi {
				w.grpcDisconnect()
			}

			// Always re-label after a re-config event. This way the new config
			// comes into effect even if the sleep interval is long (or infinite)
			labelTrigger.Reset(w.config.Core.SleepInterval.Duration)
			err = w.runFeatureDiscovery()
			if err != nil {
				return err
			}

		case <-w.certWatch.Events:
			klog.InfoS("TLS certificate update, renewing connection to nfd-master")
			w.grpcDisconnect()

		case <-w.stop:
			klog.InfoS("shutting down nfd-worker")
			if w.healthServer != nil {
				w.healthServer.GracefulStop()
			}
			configWatch.Close()
			w.certWatch.Close()
			return nil
		}
	}
}

// Stop NfdWorker
func (w *nfdWorker) Stop() {
	close(w.stop)
}

// getGrpcClient returns client connection to the NFD gRPC server. It creates a
// connection if one hasn't yet been established,.
func (w *nfdWorker) getGrpcClient() (pb.LabelerClient, error) {
	if w.grpcClient != nil {
		return w.grpcClient, nil
	}

	// Check that if a connection already exists
	if w.clientConn != nil {
		return nil, fmt.Errorf("client connection already exists")
	}

	// Dial and create a client
	dialCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dialOpts := []grpc.DialOption{grpc.WithBlock()} //nolint:staticcheck // WithBlock is deprecated
	if w.args.CaFile != "" || w.args.CertFile != "" || w.args.KeyFile != "" {
		// Load client cert for client authentication
		cert, err := tls.LoadX509KeyPair(w.args.CertFile, w.args.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %v", err)
		}
		// Load CA cert for server cert verification
		caCert, err := os.ReadFile(w.args.CaFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read root certificate file: %v", err)
		}
		caPool := x509.NewCertPool()
		if ok := caPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to add certificate from '%s'", w.args.CaFile)
		}
		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
			ServerName:   w.args.ServerNameOverride,
			MinVersion:   tls.VersionTLS13,
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	klog.InfoS("connecting to nfd-master", "address", w.args.Server)
	conn, err := grpc.DialContext(dialCtx, w.args.Server, dialOpts...) //nolint:staticcheck // DialContext is deprecated
	if err != nil {
		return nil, err
	}
	w.clientConn = conn

	w.grpcClient = pb.NewLabelerClient(w.clientConn)

	return w.grpcClient, nil
}

// grpcDisconnect closes the gRPC connection to NFD master
func (w *nfdWorker) grpcDisconnect() {
	if w.clientConn != nil {
		klog.InfoS("closing connection to nfd-master")
		w.clientConn.Close()
	}
	w.clientConn = nil
	w.grpcClient = nil
}
func (c *coreConfig) sanitize() {
	if c.SleepInterval.Duration > 0 && c.SleepInterval.Duration < time.Second {
		klog.InfoS("too short sleep interval specified, forcing to 1s",
			"sleepInterval", c.SleepInterval.Duration.String())
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

	w.featureSources = maps.Values(featureSources)

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

	w.labelSources = maps.Values(labelSources)

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
		labelsFromSource, err := getFeatureLabels(source, labelWhiteList)
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
func getFeatureLabels(source source.LabelSource, labelWhiteList regexp.Regexp) (labels Labels, err error) {
	labels = Labels{}
	features, err := source.GetLabels()
	if err != nil {
		return nil, err
	}

	for k, v := range features {
		name := k
		switch sourceName := source.Name(); sourceName {
		case "local", "custom":
			// No mangling of labels from the custom rules, hooks or feature files
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
	if features.NFDFeatureGate.Enabled(features.NodeFeatureAPI) && w.args.EnableNodeFeatureApi {
		// Create/update NodeFeature CR object
		if err := w.updateNodeFeatureObject(labels); err != nil {
			return fmt.Errorf("failed to advertise features (via CRD API): %w", err)
		}
	} else {
		// Create/update feature labels through gRPC connection to nfd-master
		if err := w.advertiseFeatureLabels(labels); err != nil {
			return fmt.Errorf("failed to advertise features (via gRPC): %w", err)
		}
	}
	return nil
}

// advertiseFeatureLabels advertises the feature labels to a Kubernetes node
// via the NFD server.
func (w *nfdWorker) advertiseFeatureLabels(labels Labels) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	klog.InfoS("sending labeling request to nfd-master")

	labelReq := pb.SetLabelsRequest{Labels: labels,
		Features:   source.GetAllFeatures(),
		NfdVersion: version.Get(),
		NodeName:   utils.NodeName()}

	cli, err := w.getGrpcClient()
	if err != nil {
		return err
	}

	_, err = cli.SetLabels(ctx, &labelReq)
	if err != nil {
		klog.ErrorS(err, "failed to label node")
		return err
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
func (m *nfdWorker) getNfdClient() (*nfdclient.Clientset, error) {
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
