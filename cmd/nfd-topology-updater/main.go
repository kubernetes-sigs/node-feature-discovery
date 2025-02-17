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

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"k8s.io/klog/v2"

	topology "sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName       = "nfd-topology-updater"
	kubeletSecurePort = 10250
)

var DefaultKubeletStateDir = path.Join(string(hostpath.VarDir), "lib", "kubelet")

func main() {
	flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	args, resourcemonitorArgs := parseArgs(flags, os.Args[1:]...)

	// Assert that the version is known
	if version.Undefined() {
		klog.InfoS("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always --match 'v*'`\" during build or run.")
	}

	// Get new TopologyUpdater instance
	instance, err := topology.NewTopologyUpdater(*args, *resourcemonitorArgs)
	if err != nil {
		klog.ErrorS(err, "failed to initialize topology updater instance")
		os.Exit(1)
	}

	if err = instance.Run(); err != nil {
		klog.ErrorS(err, "error while running")
		os.Exit(1)
	}
}

func parseArgs(flags *flag.FlagSet, osArgs ...string) (*topology.Args, *resourcemonitor.Args) {
	args, resourcemonitorArgs := initFlags(flags)
	printVersion := flags.Bool("version", false, "Print version and exit.")

	_ = flags.Parse(osArgs)
	if len(flags.Args()) > 0 {
		fmt.Fprintf(flags.Output(), "unknown command line argument: %s\n", flags.Args()[0])
		flags.Usage()
		os.Exit(2)
	}

	if *printVersion {
		fmt.Println(ProgramName, version.Get())
		os.Exit(0)
	}

	if len(resourcemonitorArgs.KubeletConfigURI) == 0 {
		nodeAddress := os.Getenv("NODE_ADDRESS")
		if len(nodeAddress) == 0 {
			fmt.Fprintf(flags.Output(), "unable to determine the default kubelet config endpoint 'https://${NODE_ADDRESS}:%d/configz' due to empty NODE_ADDRESS environment, "+
				"please either define the NODE_ADDRESS environment variable or specify endpoint with the -kubelet-config-uri flag\n", kubeletSecurePort)
			os.Exit(1)
		}
		if isIPv6(nodeAddress) {
			// With IPv6 we need to wrap the IP address in brackets as we append :port below
			nodeAddress = "[" + nodeAddress + "]"
		}
		resourcemonitorArgs.KubeletConfigURI = fmt.Sprintf("https://%s:%d/configz", nodeAddress, kubeletSecurePort)
	}

	return args, resourcemonitorArgs
}

func initFlags(flagset *flag.FlagSet) (*topology.Args, *resourcemonitor.Args) {
	args := &topology.Args{}
	resourcemonitorArgs := &resourcemonitor.Args{}

	flagset.BoolVar(&args.Oneshot, "oneshot", false,
		"Update once and exit")
	flagset.BoolVar(&args.NoPublish, "no-publish", false,
		"Do not create or update NodeResourceTopology objects.")
	flagset.StringVar(&args.KubeConfigFile, "kubeconfig", "",
		"Kube config file.")
	flagset.IntVar(&args.Port, "port", 8080,
		"Port which metrics and healthz endpoints are served on")
	flagset.DurationVar(&resourcemonitorArgs.SleepInterval, "sleep-interval", time.Duration(60)*time.Second,
		"Time to sleep between CR updates. zero means no CR updates on interval basis. [Default: 60s]")
	flagset.StringVar(&resourcemonitorArgs.Namespace, "watch-namespace", "*",
		"Namespace to watch pods (for testing/debugging purpose). Use * for all namespaces.")
	flagset.StringVar(&resourcemonitorArgs.KubeletConfigURI, "kubelet-config-uri", "",
		"Kubelet config URI path. Default to kubelet configz endpoint.")
	flagset.StringVar(&resourcemonitorArgs.APIAuthTokenFile, "api-auth-token-file", "/var/run/secrets/kubernetes.io/serviceaccount/token",
		"API auth token file path. It is used to request kubelet configz endpoint, only takes effect when kubelet-config-uri is https. Default to /var/run/secrets/kubernetes.io/serviceaccount/token.")
	flagset.StringVar(&resourcemonitorArgs.PodResourceSocketPath, "podresources-socket", hostpath.VarDir.Path("lib/kubelet/pod-resources/kubelet.sock"),
		"Pod Resource Socket path to use.")
	flagset.StringVar(&args.ConfigFile, "config", "/etc/kubernetes/node-feature-discovery/nfd-topology-updater.conf",
		"Config file to use.")
	flagset.BoolVar(&resourcemonitorArgs.PodSetFingerprint, "pods-fingerprint", true, "Compute and report the pod set fingerprint")
	flagset.StringVar(&args.KubeletStateDir, "kubelet-state-dir", DefaultKubeletStateDir, "Kubelet state directory path for watching state and checkpoint files")

	klog.InitFlags(flagset)

	return args, resourcemonitorArgs
}

func isIPv6(addr string) bool {
	ip := net.ParseIP(addr)
	return ip != nil && strings.Count(ip.String(), ":") >= 2
}
