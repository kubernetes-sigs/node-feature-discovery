/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"log"
	"time"

	"github.com/docopt/docopt-go"
	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/dumpobject"
	"sigs.k8s.io/node-feature-discovery/pkg/kubeconf"
	topology "sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/pkg/podres"
	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-topology-updater"
)

func main() {
	// Assert that the version is known
	if version.Undefined() {
		log.Printf("WARNING: version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}

	args, resourcemonitorArgs, err := argsParse(nil)
	if err != nil {
		log.Fatalf("failed to parse command line: %v", err)
	}

	klConfig, err := kubeconf.GetKubeletConfigFromLocalFile(resourcemonitorArgs.KubeletConfigFile)
	if err != nil {
		log.Fatalf("error getting topology Manager Policy: %v", err)
	}
	tmPolicy := klConfig.TopologyManagerPolicy
	log.Printf("Detected kubelet Topology Manager policy %q", tmPolicy)

	podResClient, err := podres.GetPodResClient(resourcemonitorArgs.PodResourceSocketPath)
	if err != nil {
		log.Fatalf("Failed to get PodResource Client: %v", err)
	}
	var resScan resourcemonitor.ResourcesScanner

	resScan, err = resourcemonitor.NewPodResourcesScanner(resourcemonitorArgs.Namespace, podResClient)
	if err != nil {
		log.Fatalf("Failed to initialize ResourceMonitor instance: %v", err)
	}

	// CAUTION: these resources are expected to change rarely - if ever.
	//So we are intentionally do this once during the process lifecycle.
	//TODO: Obtain node resources dynamically from the podresource API
	zonesChannel := make(chan v1alpha1.ZoneList)
	var zones v1alpha1.ZoneList

	resAggr, err := resourcemonitor.NewResourcesAggregator(resourcemonitorArgs.SysfsRoot, podResClient)
	if err != nil {
		log.Fatalf("Failed to obtain node resource information: %v", err)
	}

	log.Printf("resAggr is: %v\n", resAggr)
	go func() {
		for {
			log.Printf("Scanning\n")

			podResources, err := resScan.Scan()
			log.Printf("podResources are: %v\n", podResources)
			if err != nil {
				log.Printf("Scan failed: %v\n", err)
				continue
			}

			zones = resAggr.Aggregate(podResources)
			zonesChannel <- zones
			log.Printf("After aggregating resources identified zones are:%v", dumpobject.DumpObject(zones))

			time.Sleep(resourcemonitorArgs.SleepInterval)
		}
	}()

	// Get new TopologyUpdater instance
	instance, err := topology.NewTopologyUpdater(args, tmPolicy)
	if err != nil {
		log.Fatalf("Failed to initialize NfdWorker instance: %v", err)
	}
	for {

		zonesValue := <-zonesChannel
		log.Printf("Received value on ZoneChannel\n")
		if err = instance.Update(zonesValue); err != nil {
			log.Fatalf("ERROR: %v", err)
		}
		if args.Oneshot {
			break
		}
	}
}

// argsParse parses the command line arguments passed to the program.
// The argument argv is passed only for testing purposes.
func argsParse(argv []string) (topology.Args, resourcemonitor.Args, error) {
	args := topology.Args{}
	resourcemonitorArgs := resourcemonitor.Args{}
	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish] [--oneshot | --sleep-interval=<seconds>] [--server=<server>]
	   [--server-name-override=<name>] [--ca-file=<path>] [--cert-file=<path>]
		 [--key-file=<path>] [--container-runtime=<runtime>] [--podresources-socket=<path>]
		 [--watch-namespace=<namespace>] [--sysfs=<mountpoint>] [--kubelet-config-file=<path>]

  %s -h | --help
  %s --version

  Options:
  -h --help                       Show this screen.
  --version                       Output version and exit.
  --ca-file=<path>                Root certificate for verifying connections
                                  [Default: ]
  --cert-file=<path>              Certificate used for authenticating connections
                                  [Default: ]
  --key-file=<path>               Private key matching --cert-file
                                  [Default: ]
  --server=<server>               NFD server address to connect to.
                                  [Default: localhost:8080]
  --server-name-override=<name>   Name (CN) expect from server certificate, useful
                                  in testing
                                  [Default: ]
  --no-publish                    Do not publish discovered features to the
                                  cluster-local Kubernetes API server.
  --oneshot                       Update once and exit.
  --sleep-interval=<seconds>      Time to sleep between re-labeling. Non-positive
                                  value implies no re-labeling (i.e. infinite
                                  sleep). [Default: 60s]
  --watch-namespace=<namespace>   Namespace to watch pods for. Use "" for all namespaces.
  --sysfs=<mountpoint>            Mount point of the sysfs.
                                  [Default: /host]
  --kubelet-config-file=<path>    Kubelet config file path.
                                  [Default: /podresources/config.yaml]
  --podresources-socket=<path>    Pod Resource Socket path to use.
                                  [Default: /podresources/kubelet.sock] `,

		ProgramName,
		ProgramName,
		ProgramName,
		ProgramName,
	)

	arguments, _ := docopt.ParseArgs(usage, argv,
		fmt.Sprintf("%s %s", ProgramName, version.Get()))

	// Parse argument values as usable types.
	var err error
	args.CaFile = arguments["--ca-file"].(string)
	args.CertFile = arguments["--cert-file"].(string)
	args.KeyFile = arguments["--key-file"].(string)
	args.NoPublish = arguments["--no-publish"].(bool)
	args.Server = arguments["--server"].(string)
	args.ServerNameOverride = arguments["--server-name-override"].(string)
	args.Oneshot = arguments["--oneshot"].(bool)
	resourcemonitorArgs.SleepInterval, err = time.ParseDuration(arguments["--sleep-interval"].(string))
	if err != nil {
		return args, resourcemonitorArgs, fmt.Errorf("invalid --sleep-interval specified: %s", err.Error())
	}
	if ns, ok := arguments["--watch-namespace"].(string); ok {
		resourcemonitorArgs.Namespace = ns
	}
	if kubeletConfigPath, ok := arguments["--kubelet-config-file"].(string); ok {
		resourcemonitorArgs.KubeletConfigFile = kubeletConfigPath
	}
	resourcemonitorArgs.SysfsRoot = arguments["--sysfs"].(string)
	if path, ok := arguments["--podresources-socket"].(string); ok {
		resourcemonitorArgs.PodResourceSocketPath = path
	}

	return args, resourcemonitorArgs, nil
}
