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

package topologyupdater

import (
	"fmt"
	"time"

	"k8s.io/klog/v2"

	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"golang.org/x/net/context"

	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/nfd-client"
	"sigs.k8s.io/node-feature-discovery/pkg/podres"
	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
	pb "sigs.k8s.io/node-feature-discovery/pkg/topologyupdater"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// Command line arguments
type Args struct {
	nfdclient.Args
	NoPublish      bool
	Oneshot        bool
	KubeConfigFile string
}

type NfdTopologyUpdater interface {
	nfdclient.NfdClient
	Update(v1alpha1.ZoneList) error
}

type staticNodeInfo struct {
	tmPolicy string
}

type nfdTopologyUpdater struct {
	nfdclient.NfdBaseClient
	nodeInfo            *staticNodeInfo
	args                Args
	resourcemonitorArgs resourcemonitor.Args
	certWatch           *utils.FsWatcher
	client              pb.NodeTopologyClient
	stop                chan struct{} // channel for signaling stop
}

// Create new NewTopologyUpdater instance.
func NewTopologyUpdater(args Args, resourcemonitorArgs resourcemonitor.Args, policy string) (NfdTopologyUpdater, error) {
	base, err := nfdclient.NewNfdBaseClient(&args.Args)
	if err != nil {
		return nil, err
	}

	nfd := &nfdTopologyUpdater{
		NfdBaseClient:       base,
		args:                args,
		resourcemonitorArgs: resourcemonitorArgs,
		nodeInfo: &staticNodeInfo{
			tmPolicy: policy,
		},
		stop: make(chan struct{}, 1),
	}
	return nfd, nil
}

// Run nfdTopologyUpdater client. Returns if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the updater args.
func (w *nfdTopologyUpdater) Run() error {
	klog.Infof("Node Feature Discovery Topology Updater %s", version.Get())
	klog.Infof("NodeName: '%s'", nfdclient.NodeName())

	podResClient, err := podres.GetPodResClient(w.resourcemonitorArgs.PodResourceSocketPath)
	if err != nil {
		return fmt.Errorf("failed to get PodResource Client: %w", err)
	}

	var kubeApihelper apihelper.K8sHelpers
	if !w.args.NoPublish {
		kubeconfig, err := apihelper.GetKubeconfig(w.args.KubeConfigFile)
		if err != nil {
			return err
		}
		kubeApihelper = apihelper.K8sHelpers{Kubeconfig: kubeconfig}
	}

	var resScan resourcemonitor.ResourcesScanner

	resScan, err = resourcemonitor.NewPodResourcesScanner(w.resourcemonitorArgs.Namespace, podResClient, kubeApihelper)
	if err != nil {
		return fmt.Errorf("failed to initialize ResourceMonitor instance: %w", err)
	}

	// CAUTION: these resources are expected to change rarely - if ever.
	// So we are intentionally do this once during the process lifecycle.
	// TODO: Obtain node resources dynamically from the podresource API
	// zonesChannel := make(chan v1alpha1.ZoneList)
	var zones v1alpha1.ZoneList

	resAggr, err := resourcemonitor.NewResourcesAggregator(podResClient)
	if err != nil {
		return fmt.Errorf("failed to obtain node resource information: %w", err)
	}

	klog.V(2).Infof("resAggr is: %v\n", resAggr)

	// Create watcher for TLS certificates
	w.certWatch, err = utils.CreateFsWatcher(time.Second, w.args.CaFile, w.args.CertFile, w.args.KeyFile)
	if err != nil {
		return err
	}

	crTrigger := time.After(0)
	for {
		select {
		case <-crTrigger:
			klog.Infof("Scanning\n")
			podResources, err := resScan.Scan()
			utils.KlogDump(1, "podResources are", "  ", podResources)
			if err != nil {
				klog.Warningf("Scan failed: %v\n", err)
				continue
			}
			zones = resAggr.Aggregate(podResources)
			utils.KlogDump(1, "After aggregating resources identified zones are", "  ", zones)
			if err = w.Update(zones); err != nil {
				return err
			}

			if w.args.Oneshot {
				return nil
			}

			if w.resourcemonitorArgs.SleepInterval > 0 {
				crTrigger = time.After(w.resourcemonitorArgs.SleepInterval)
			}

		case <-w.certWatch.Events:
			klog.Infof("TLS certificate update, renewing connection to nfd-master")
			w.Disconnect()
			if err := w.Connect(); err != nil {
				return err
			}

		case <-w.stop:
			klog.Infof("shutting down nfd-topology-updater")
			w.certWatch.Close()
			return nil
		}
	}

}

func (w *nfdTopologyUpdater) Update(zones v1alpha1.ZoneList) error {
	// Connect to NFD master
	err := w.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer w.Disconnect()

	if w.client == nil {
		return nil
	}

	err = advertiseNodeTopology(w.client, zones, w.nodeInfo.tmPolicy, nfdclient.NodeName())
	if err != nil {
		return fmt.Errorf("failed to advertise node topology: %w", err)
	}
	return nil
}

// Stop NFD Topology Updater
func (w *nfdTopologyUpdater) Stop() {
	select {
	case w.stop <- struct{}{}:
	default:
	}
}

// connect creates a client connection to the NFD master
func (w *nfdTopologyUpdater) Connect() error {
	// Return a dummy connection in case of dry-run
	if w.args.NoPublish {
		return nil
	}

	if err := w.NfdBaseClient.Connect(); err != nil {
		return err
	}
	w.client = pb.NewNodeTopologyClient(w.ClientConn())

	return nil
}

// disconnect closes the connection to NFD master
func (w *nfdTopologyUpdater) Disconnect() {
	w.NfdBaseClient.Disconnect()
	w.client = nil
}

// advertiseNodeTopology advertises the topology CR to a Kubernetes node
// via the NFD server.
func advertiseNodeTopology(client pb.NodeTopologyClient, zoneInfo v1alpha1.ZoneList, tmPolicy string, nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	zones := make([]*v1alpha1.Zone, len(zoneInfo))
	// TODO: Avoid copying of data to allow returning the zone info
	// directly in a compatible data type (i.e. []*v1alpha1.Zone).
	for i, zone := range zoneInfo {
		zones[i] = &v1alpha1.Zone{
			Name:      zone.Name,
			Type:      zone.Type,
			Parent:    zone.Parent,
			Resources: zone.Resources,
			Costs:     zone.Costs,
		}
	}

	topologyReq := &pb.NodeTopologyRequest{
		Zones:            zones,
		NfdVersion:       version.Get(),
		NodeName:         nodeName,
		TopologyPolicies: []string{tmPolicy},
	}

	utils.KlogDump(1, "Sending NodeTopologyRequest to nfd-master:", "  ", topologyReq)

	_, err := client.UpdateNodeTopology(ctx, topologyReq)
	if err != nil {
		return err
	}

	return nil
}
