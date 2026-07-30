/*
Copyright 2021 The Kubernetes Authors.

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

package nfdtopologyupdater

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/informers"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	applyconfigurationtopologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/applyconfiguration/topology/v1alpha2"
	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater/kubeletnotifier"
	"sigs.k8s.io/node-feature-discovery/pkg/podres"
	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
	"sigs.k8s.io/node-feature-discovery/pkg/topologypolicy"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/kubeconf"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/yaml"
)

// nodeScopedPodInformerResync is zero because topology scans read pods
// from the cached lister only. This value only makes sense when event
// handlers are registered to the informer.
const nodeScopedPodInformerResync = 0

const (
	topologyManagerInfoRefreshTimeout = 2 * time.Minute

	// TopologyManagerPolicyAttributeName represents an attribute which defines
	// Topology Manager Policy
	TopologyManagerPolicyAttributeName = "topologyManagerPolicy"
	// TopologyManagerScopeAttributeName represents an attribute which defines
	// Topology Manager Policy Scope
	TopologyManagerScopeAttributeName = "topologyManagerScope"
	// NodeResourceTopologyCRDName is the name of the NodeResourceTopology CRD
	NodeResourceTopologyCRDName = "noderesourcetopologies.topology.node.k8s.io"
	// nodeResourceTopologyFieldManager is used for server-side apply updates to
	// NodeResourceTopology objects.
	nodeResourceTopologyFieldManager = "node-feature-discovery-topology-updater"
)

var errTopologyManagerInfoUnavailable = errors.New("topology-manager information unavailable")

// Args are the command line arguments
type Args struct {
	Port            int
	NoPublish       bool
	Oneshot         bool
	KubeConfigFile  string
	ConfigFile      string
	KubeletStateDir string

	Klog map[string]*utils.KlogFlagVal
}

// NFDConfig contains the configuration settings of NFDTopologyUpdater.
type NFDConfig struct {
	ExcludeList map[string][]string
}

type NfdTopologyUpdater interface {
	Run() error
	Stop()
}

type nfdTopologyUpdater struct {
	nodeName             string
	args                 Args
	topoClient           topologyclientset.Interface
	resourcemonitorArgs  resourcemonitor.Args
	stop                 chan struct{} // channel for signaling stop
	eventSource          <-chan kubeletnotifier.Info
	configFilePath       string
	config               *NFDConfig
	kubernetesNamespace  string
	ownerRefs            []metav1.OwnerReference
	k8sClient            k8sclient.Interface
	kubeletConfigFunc    func(context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error)
	discoverCpuCoresFunc func() v1alpha2.AttributeList
	kubeletConfigMu      sync.RWMutex
	kubeletConfig        *kubeletconfigv1beta1.KubeletConfiguration
	cpuCoreAttributes    v1alpha2.AttributeList
}

// NewTopologyUpdater creates a new NfdTopologyUpdater instance.
func NewTopologyUpdater(args Args, resourcemonitorArgs resourcemonitor.Args) (NfdTopologyUpdater, error) {
	eventSource := make(chan kubeletnotifier.Info)

	ntf, err := kubeletnotifier.New(resourcemonitorArgs.SleepInterval, eventSource, args.KubeletStateDir)
	if err != nil {
		return nil, err
	}
	go ntf.Run()

	kubeletConfigFunc, err := getKubeletConfigFunc(resourcemonitorArgs.KubeletConfigURI, resourcemonitorArgs.APIAuthTokenFile)
	if err != nil {
		return nil, err
	}

	nfd := &nfdTopologyUpdater{
		args:                 args,
		resourcemonitorArgs:  resourcemonitorArgs,
		stop:                 make(chan struct{}),
		nodeName:             utils.NodeName(),
		eventSource:          eventSource,
		config:               &NFDConfig{},
		kubernetesNamespace:  utils.GetKubernetesNamespace(),
		ownerRefs:            []metav1.OwnerReference{},
		kubeletConfigFunc:    kubeletConfigFunc,
		discoverCpuCoresFunc: discoverCpuCores,
	}
	if args.ConfigFile != "" {
		nfd.configFilePath = filepath.Clean(args.ConfigFile)
	}
	return nfd, nil
}

// refreshTopologyManagerInfo fetches and caches the inputs used to publish
// topology-manager information.
func (w *nfdTopologyUpdater) refreshTopologyManagerInfo(ctx context.Context) error {
	klConfig, err := w.kubeletConfigFunc(ctx)
	if err != nil {
		return err
	}
	cpuCoreAttributes := w.discoverCpuCoresFunc()

	w.kubeletConfigMu.Lock()
	w.kubeletConfig = klConfig
	w.cpuCoreAttributes = cpuCoreAttributes
	w.kubeletConfigMu.Unlock()
	return nil
}

// getKubeletConfig returns the cached kubelet configuration, or nil if it has
// not been fetched yet.
func (w *nfdTopologyUpdater) getKubeletConfig() *kubeletconfigv1beta1.KubeletConfiguration {
	w.kubeletConfigMu.RLock()
	defer w.kubeletConfigMu.RUnlock()
	return w.kubeletConfig
}

func (w *nfdTopologyUpdater) getCpuCoreAttributes() v1alpha2.AttributeList {
	w.kubeletConfigMu.RLock()
	defer w.kubeletConfigMu.RUnlock()
	return append(v1alpha2.AttributeList(nil), w.cpuCoreAttributes...)
}

func (w *nfdTopologyUpdater) newTopologyManagerRefreshContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), topologyManagerInfoRefreshTimeout)
	go func() {
		select {
		case <-w.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (w *nfdTopologyUpdater) detectTopologyPolicyAndScope() (string, string, error) {
	klConfig := w.getKubeletConfig()
	if klConfig == nil {
		return "", "", fmt.Errorf("kubelet configuration is not cached")
	}

	return klConfig.TopologyManagerPolicy, klConfig.TopologyManagerScope, nil
}

func (w *nfdTopologyUpdater) Healthz(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

// Run nfdTopologyUpdater. Returns if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the updater args.
func (w *nfdTopologyUpdater) Run() error {
	klog.InfoS("Node Feature Discovery Topology Updater", "version", version.Get(), "nodeName", w.nodeName)

	// Start HTTP server early so health probes work during initialization.
	// This is important because we may wait for the CRD to be available.
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", w.Healthz)

	// Register to metrics server
	promRegistry := prometheus.NewRegistry()
	promRegistry.MustRegister(
		buildInfo,
		scanErrors)
	httpMux.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}))
	registerVersion(version.Get())

	httpServer := http.Server{Addr: fmt.Sprintf(":%d", w.args.Port), Handler: httpMux}
	go func() {
		klog.InfoS("http server starting", "port", httpServer.Addr)
		klog.InfoS("http server stopped", "exitCode", httpServer.ListenAndServe())
	}()
	defer httpServer.Close() // nolint: errcheck

	podResClient, err := podres.GetPodResClient(w.resourcemonitorArgs.PodResourceSocketPath)
	if err != nil {
		return fmt.Errorf("failed to get PodResource Client: %w", err)
	}

	kubeconfig, err := utils.GetKubeconfig(w.args.KubeConfigFile)
	if err != nil {
		return err
	}
	topoClient, err := topologyclientset.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create topology client: %w", err)
	}
	w.topoClient = topoClient

	k8sClient, err := k8sclient.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	w.k8sClient = k8sClient

	// Wait for the NodeResourceTopology CRD to be available before proceeding.
	// This handles race conditions during deployment and scenarios where the
	// CRD is installed after the topology-updater.
	if err := waitForNodeResourceTopologyCRD(kubeconfig, w.stop); err != nil {
		return err
	}

	if err := w.configure(); err != nil {
		return fmt.Errorf("faild to configure Node Feature Discovery Topology Updater: %w", err)
	}

	// Cache topology-manager information once on startup. It is subsequently
	// refreshed on interval-based events (see the event loop below).
	refreshCtx, cancelRefresh := w.newTopologyManagerRefreshContext()
	err = w.refreshTopologyManagerInfo(refreshCtx)
	cancelRefresh()
	if err != nil {
		klog.ErrorS(err, "failed to cache topology-manager information on startup, will retry on next scan")
	}

	syncCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Build a node-scoped Pod informer so per-pod lookups in
	// PodResourcesScanner.isWatchable() are served from a local cache
	// avoiding cluster-wide pod traffic.
	podInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		k8sClient,
		nodeScopedPodInformerResync,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", w.nodeName).String()
		}),
	)
	podInformer := podInformerFactory.Core().V1().Pods()
	podLister := podInformer.Lister()

	podInformerStop := make(chan struct{})
	var stopPodInformerOnce sync.Once
	stopPodInformer := func() {
		stopPodInformerOnce.Do(func() {
			close(podInformerStop)
		})
	}
	defer stopPodInformer()

	go func() {
		select {
		case <-w.stop:
			cancel()
			stopPodInformer()
		case <-podInformerStop:
		}
	}()

	podInformerFactory.Start(podInformerStop)
	klog.InfoS("waiting for node-scoped Pod informer cache sync", "nodeName", w.nodeName)
	if !cache.WaitForCacheSync(syncCtx.Done(), podInformer.Informer().HasSynced) {
		return fmt.Errorf("timed out waiting for node-scoped Pod informer cache sync for node %q", w.nodeName)
	}
	klog.InfoS("node-scoped Pod informer cache synced", "nodeName", w.nodeName)

	var resScan resourcemonitor.ResourcesScanner

	resScan, err = resourcemonitor.NewPodResourcesScanner(w.resourcemonitorArgs.Namespace, podResClient, podLister, k8sClient, w.resourcemonitorArgs.PodSetFingerprint)
	if err != nil {
		return fmt.Errorf("failed to initialize ResourceMonitor instance: %w", err)
	}

	// CAUTION: these resources are expected to change rarely - if ever.
	// So we are intentionally do this once during the process lifecycle.
	// TODO: Obtain node resources dynamically from the podresource API
	var zones v1alpha2.ZoneList

	excludeList := resourcemonitor.NewExcludeResourceList(w.config.ExcludeList, w.nodeName)
	resAggr, err := resourcemonitor.NewResourcesAggregator(podResClient, excludeList)
	if err != nil {
		return fmt.Errorf("failed to obtain node resource information: %w", err)
	}

	for {
		select {
		case info := <-w.eventSource:
			klog.V(4).InfoS("event received, scanning...", "event", info.Event)
			scanResponse, err := resScan.Scan()
			klog.V(1).InfoS("received updated pod resources", "podResources", utils.DelayedDumper(scanResponse.PodResources))
			if err != nil {
				klog.ErrorS(err, "scan failed")
				scanErrors.Inc()
				continue
			}
			zones = resAggr.Aggregate(scanResponse.PodResources)
			klog.V(1).InfoS("aggregated resources identified", "resourceZones", utils.DelayedDumper(zones))
			if info.Event == kubeletnotifier.IntervalBased {
				refreshCtx, cancel := w.newTopologyManagerRefreshContext()
				err := w.refreshTopologyManagerInfo(refreshCtx)
				cancel()
				if err != nil {
					klog.ErrorS(err, "failed to refresh topology-manager information, keeping cached information")
				}
			}

			if !w.args.NoPublish {
				if err = w.updateNodeResourceTopology(zones, scanResponse); err != nil {
					if errors.Is(err, errTopologyManagerInfoUnavailable) {
						scanErrors.Inc()
						klog.ErrorS(err, "skipping NodeResourceTopology publish")
						if w.args.Oneshot {
							return err
						}
						continue
					}
					return err
				}
			}

			if w.args.Oneshot {
				return nil
			}

		case <-w.stop:
			klog.InfoS("shutting down nfd-topology-updater")
			return nil
		}
	}

}

// Stop NFD Topology Updater
func (w *nfdTopologyUpdater) Stop() {
	close(w.stop)
}

func (w *nfdTopologyUpdater) updateNodeResourceTopology(zoneInfo v1alpha2.ZoneList, scanResponse resourcemonitor.ScanResponse) error {

	if len(w.ownerRefs) == 0 {
		ns, err := w.k8sClient.CoreV1().Namespaces().Get(context.TODO(), w.kubernetesNamespace, metav1.GetOptions{})
		if err != nil {
			klog.ErrorS(err, "Cannot get NodeResourceTopology owner reference")
		} else {
			w.ownerRefs = []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Namespace",
					Name:       ns.Name,
					UID:        types.UID(ns.UID),
				},
			}
		}
	}

	nrt := &v1alpha2.NodeResourceTopology{
		Attributes: v1alpha2.AttributeList{},
	}

	if err := w.applyNRTTopologyManagerInfo(nrt); err != nil {
		return fmt.Errorf("%w: %w", errTopologyManagerInfoUnavailable, err)
	}

	updateAttributes(&nrt.Attributes, scanResponse.Attributes)

	nrtApply := applyconfigurationtopologyv1alpha2.NodeResourceTopology(w.nodeName)
	if len(w.ownerRefs) > 0 {
		ownerRefApplies := make([]*metav1apply.OwnerReferenceApplyConfiguration, len(w.ownerRefs))
		for i, ref := range w.ownerRefs {
			ownerRefApplies[i] = metav1apply.OwnerReference().
				WithAPIVersion(ref.APIVersion).
				WithKind(ref.Kind).
				WithName(ref.Name).
				WithUID(ref.UID)
		}
		nrtApply.WithOwnerReferences(ownerRefApplies...)
	}
	nrtApply.WithZones(zoneInfo).
		WithAttributes(nrt.Attributes).
		WithTopologyPolicies(nrt.TopologyPolicies...)

	nrtUpdated, err := w.topoClient.TopologyV1alpha2().NodeResourceTopologies().Apply(
		context.TODO(),
		nrtApply,
		metav1.ApplyOptions{
			FieldManager: nodeResourceTopologyFieldManager,
			Force:        true,
		},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to apply NodeResourceTopology: %w. "+
				"The NodeResourceTopology CRD may not be installed. "+
				"If using Helm, ensure 'topologyUpdater.createCRDs=true' is set",
				err)
		}
		return fmt.Errorf("failed to apply NodeResourceTopology: %w", err)
	}

	klog.V(4).InfoS("NodeResourceTopology object applied", "nodeResourceTopology", utils.DelayedDumper(nrtUpdated))
	return nil
}

func (w *nfdTopologyUpdater) applyNRTTopologyManagerInfo(nrt *v1alpha2.NodeResourceTopology) error {
	if w.getKubeletConfig() == nil {
		refreshCtx, cancel := w.newTopologyManagerRefreshContext()
		err := w.refreshTopologyManagerInfo(refreshCtx)
		cancel()
		if err != nil {
			return err
		}
	}

	return w.updateNRTTopologyManagerInfo(nrt)
}

// Discover E/P cores
func discoverCpuCores() v1alpha2.AttributeList {
	attrList := v1alpha2.AttributeList{}

	cpusPathGlob := hostpath.SysfsDir.Path("sys/devices/cpu_*/cpus")
	cpuPaths, err := filepath.Glob(cpusPathGlob)
	if err != nil {
		klog.ErrorS(err, "error reading cpu entries", "cpusPathGlob", cpusPathGlob)
		return attrList
	}

	for _, entry := range cpuPaths {
		cpus, err := os.ReadFile(entry)
		if err != nil {
			klog.ErrorS(err, "error reading cpu entry file", "entry", entry)
		} else {
			attrList = append(attrList, v1alpha2.AttributeInfo{
				Name:  filepath.Base(filepath.Dir(entry)),
				Value: strings.TrimSpace(string(cpus)),
			})
		}
	}

	return attrList
}

func (w *nfdTopologyUpdater) updateNRTTopologyManagerInfo(nrt *v1alpha2.NodeResourceTopology) error {
	policy, scope, err := w.detectTopologyPolicyAndScope()
	if err != nil {
		return fmt.Errorf("failed to detect TopologyManager's policy and scope: %w", err)
	}

	tmAttributes := createTopologyAttributes(policy, scope)
	deprecatedTopologyPolicies := []string{string(topologypolicy.DetectTopologyPolicy(policy, scope))}

	updateAttributes(&nrt.Attributes, tmAttributes)
	nrt.TopologyPolicies = deprecatedTopologyPolicies

	attrList := w.getCpuCoreAttributes()
	updateAttributes(&nrt.Attributes, attrList)

	return nil
}

func (w *nfdTopologyUpdater) configure() error {
	if w.configFilePath == "" {
		klog.InfoS("no configuration file specified")
		return nil
	}

	b, err := os.ReadFile(w.configFilePath)
	if err != nil {
		// config is optional
		if os.IsNotExist(err) {
			klog.InfoS("configuration file not found", "path", w.configFilePath)
			return nil
		}
		return err
	}

	err = yaml.Unmarshal(b, w.config)
	if err != nil {
		return fmt.Errorf("failed to parse configuration file %q: %w", w.configFilePath, err)
	}
	klog.InfoS("configuration file parsed", "path", w.configFilePath, "config", w.config)
	return nil
}

// waitForNodeResourceTopologyCRD waits for the NodeResourceTopology CRD to be
// available in the cluster. This handles race conditions during deployment and
// scenarios where the CRD is installed after the topology-updater pods start.
// The function will retry indefinitely until the CRD is found or the stop
// channel is closed.
func waitForNodeResourceTopologyCRD(config *restclient.Config, stop <-chan struct{}) error {
	const (
		initialBackoff = 5 * time.Second
		maxBackoff     = 60 * time.Second
	)

	apiextClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		klog.V(2).InfoS("unable to create apiextensions client for CRD check, skipping wait",
			"error", err)
		// Don't block startup, the error will be caught later when creating NRT
		return nil
	}

	backoff := initialBackoff
	for {
		_, err = apiextClient.ApiextensionsV1().CustomResourceDefinitions().Get(
			context.TODO(), NodeResourceTopologyCRDName, metav1.GetOptions{})
		if err == nil {
			klog.InfoS("NodeResourceTopology CRD is available",
				"crd", NodeResourceTopologyCRDName)
			return nil
		}

		// If we don't have permission to check CRDs, skip waiting and let the
		// actual NRT creation fail with a more descriptive error
		if apierrors.IsForbidden(err) {
			klog.V(2).InfoS("no permission to check CRD existence, skipping wait",
				"crd", NodeResourceTopologyCRDName, "error", err)
			return nil
		}

		if apierrors.IsNotFound(err) {
			klog.InfoS("waiting for NodeResourceTopology CRD to be created. "+
				"If using Helm, ensure 'topologyUpdater.createCRDs=true' is set",
				"crd", NodeResourceTopologyCRDName, "retryIn", backoff)
		} else {
			klog.V(2).InfoS("error checking for CRD, will retry",
				"crd", NodeResourceTopologyCRDName, "error", err, "retryIn", backoff)
		}

		select {
		case <-stop:
			return fmt.Errorf("stopped while waiting for CRD %q",
				NodeResourceTopologyCRDName)
		case <-time.After(backoff):
			// Exponential backoff with max cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func createTopologyAttributes(policy string, scope string) v1alpha2.AttributeList {
	return v1alpha2.AttributeList{
		{
			Name:  TopologyManagerPolicyAttributeName,
			Value: policy,
		},
		{
			Name:  TopologyManagerScopeAttributeName,
			Value: scope,
		},
	}
}

func updateAttribute(attrList *v1alpha2.AttributeList, attrInfo v1alpha2.AttributeInfo) {
	if attrList == nil {
		return
	}

	for idx := range *attrList {
		if (*attrList)[idx].Name == attrInfo.Name {
			(*attrList)[idx].Value = attrInfo.Value
			return
		}
	}
	*attrList = append(*attrList, attrInfo)
}
func updateAttributes(lhs *v1alpha2.AttributeList, rhs v1alpha2.AttributeList) {
	for _, attr := range rhs {
		updateAttribute(lhs, attr)
	}
}

func getKubeletConfigFunc(uri, apiAuthTokenFile string) (func(context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error), error) {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse -kubelet-config-uri: %w", err)
	}

	// init kubelet API client
	var klConfig *kubeletconfigv1beta1.KubeletConfiguration
	switch u.Scheme {
	case "file":
		return func(context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			klConfig, err = kubeconf.GetKubeletConfigFromLocalFile(u.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to read kubelet config: %w", err)
			}
			return klConfig, err
		}, nil
	case "https":
		restConfig, err := kubeconf.InsecureConfig(u.String(), apiAuthTokenFile)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize rest config for kubelet config uri: %w", err)
		}

		return func(ctx context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			klConfig, err = kubeconf.GetKubeletConfigurationWithContext(ctx, restConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to get kubelet config from configz endpoint: %w", err)
			}
			return klConfig, nil
		}, nil
	}

	return nil, fmt.Errorf("unsupported URI scheme: %v", u.Scheme)
}
