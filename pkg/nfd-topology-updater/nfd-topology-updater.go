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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
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

const (
	// TopologyManagerPolicyAttributeName represents an attribute which defines
	// Topology Manager Policy
	TopologyManagerPolicyAttributeName = "topologyManagerPolicy"
	// TopologyManagerScopeAttributeName represents an attribute which defines
	// Topology Manager Policy Scope
	TopologyManagerScopeAttributeName = "topologyManagerScope"
	// NodeResourceTopologyCRDName is the name of the NodeResourceTopology CRD
	NodeResourceTopologyCRDName = "noderesourcetopologies.topology.node.k8s.io"
)

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
	nodeName            string
	args                Args
	topoClient          topologyclientset.Interface
	resourcemonitorArgs resourcemonitor.Args
	stop                chan struct{} // channel for signaling stop
	eventSource         <-chan kubeletnotifier.Info
	configFilePath      string
	config              *NFDConfig
	kubernetesNamespace string
	ownerRefs           []metav1.OwnerReference
	k8sClient           k8sclient.Interface
	kubeletConfigFunc   func() (*kubeletconfigv1beta1.KubeletConfiguration, error)
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
		args:                args,
		resourcemonitorArgs: resourcemonitorArgs,
		stop:                make(chan struct{}),
		nodeName:            utils.NodeName(),
		eventSource:         eventSource,
		config:              &NFDConfig{},
		kubernetesNamespace: utils.GetKubernetesNamespace(),
		ownerRefs:           []metav1.OwnerReference{},
		kubeletConfigFunc:   kubeletConfigFunc,
	}
	if args.ConfigFile != "" {
		nfd.configFilePath = filepath.Clean(args.ConfigFile)
	}
	return nfd, nil
}

func (w *nfdTopologyUpdater) detectTopologyPolicyAndScope() (string, string, error) {
	klConfig, err := w.kubeletConfigFunc()
	if err != nil {
		return "", "", err
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

	var resScan resourcemonitor.ResourcesScanner

	resScan, err = resourcemonitor.NewPodResourcesScanner(w.resourcemonitorArgs.Namespace, podResClient, k8sClient, w.resourcemonitorArgs.PodSetFingerprint)
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
			readKubeletConfig := false
			if info.Event == kubeletnotifier.IntervalBased {
				readKubeletConfig = true
			}

			if !w.args.NoPublish {
				if err = w.updateNodeResourceTopology(zones, scanResponse, readKubeletConfig); err != nil {
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

func (w *nfdTopologyUpdater) updateNodeResourceTopology(zoneInfo v1alpha2.ZoneList, scanResponse resourcemonitor.ScanResponse, readKubeletConfig bool) error {

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

	nrt, err := w.topoClient.TopologyV1alpha2().NodeResourceTopologies().Get(context.TODO(), w.nodeName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nrtNew := v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name:            w.nodeName,
				OwnerReferences: w.ownerRefs,
			},
			Zones:      zoneInfo,
			Attributes: v1alpha2.AttributeList{},
		}

		if err := w.updateNRTTopologyManagerInfo(&nrtNew); err != nil {
			return err
		}

		updateAttributes(&nrtNew.Attributes, scanResponse.Attributes)

		if _, err := w.topoClient.TopologyV1alpha2().NodeResourceTopologies().Create(context.TODO(), &nrtNew, metav1.CreateOptions{}); err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to create NodeResourceTopology: %w. "+
					"The NodeResourceTopology CRD may not be installed. "+
					"If using Helm, ensure 'topologyUpdater.createCRDs=true' is set",
					err)
			}
			return fmt.Errorf("failed to create NodeResourceTopology: %w", err)
		}
		return nil
	} else if err != nil {
		return err
	}

	nrtMutated := nrt.DeepCopy()
	nrtMutated.Zones = zoneInfo
	nrtMutated.OwnerReferences = w.ownerRefs

	attributes := scanResponse.Attributes

	if readKubeletConfig {
		if err := w.updateNRTTopologyManagerInfo(nrtMutated); err != nil {
			return err
		}
	}

	updateAttributes(&nrtMutated.Attributes, attributes)

	nrtUpdated, err := w.topoClient.TopologyV1alpha2().NodeResourceTopologies().Update(context.TODO(), nrtMutated, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update NodeResourceTopology: %w", err)
	}

	klog.V(4).InfoS("NodeResourceTopology object updated", "nodeResourceTopology", utils.DelayedDumper(nrtUpdated))
	return nil
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

	attrList := discoverCpuCores()
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
		if errors.IsForbidden(err) {
			klog.V(2).InfoS("no permission to check CRD existence, skipping wait",
				"crd", NodeResourceTopologyCRDName, "error", err)
			return nil
		}

		if errors.IsNotFound(err) {
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

func getKubeletConfigFunc(uri, apiAuthTokenFile string) (func() (*kubeletconfigv1beta1.KubeletConfiguration, error), error) {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse -kubelet-config-uri: %w", err)
	}

	// init kubelet API client
	var klConfig *kubeletconfigv1beta1.KubeletConfiguration
	switch u.Scheme {
	case "file":
		return func() (*kubeletconfigv1beta1.KubeletConfiguration, error) {
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

		return func() (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			klConfig, err = kubeconf.GetKubeletConfiguration(restConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to get kubelet config from configz endpoint: %w", err)
			}
			return klConfig, nil
		}, nil
	}

	return nil, fmt.Errorf("unsupported URI scheme: %v", u.Scheme)
}
