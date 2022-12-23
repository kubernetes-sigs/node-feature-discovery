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
	"net/url"
	"os"
	"time"

	"k8s.io/klog/v2"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	topology "sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
	"sigs.k8s.io/node-feature-discovery/pkg/topologypolicy"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/kubeconf"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName       = "nfd-topology-updater"
	kubeletSecurePort = 10250
)

func main() {
	flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	printVersion := flags.Bool("version", false, "Print version and exit.")

	args, resourcemonitorArgs := parseArgs(flags, os.Args[1:]...)

	if *printVersion {
		fmt.Println(ProgramName, version.Get())
		os.Exit(0)
	}

	// Assert that the version is known
	if version.Undefined() {
		klog.Warningf("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}

	// Plug klog into grpc logging infrastructure
	utils.ConfigureGrpcKlog()

	u, err := url.ParseRequestURI(resourcemonitorArgs.KubeletConfigURI)
	if err != nil {
		klog.Exitf("failed to parse args for kubelet-config-uri: %v", err)
	}

	// init kubelet API client
	var klConfig *kubeletconfigv1beta1.KubeletConfiguration
	switch u.Scheme {
	case "file":
		klConfig, err = kubeconf.GetKubeletConfigFromLocalFile(u.Path)
		if err != nil {
			klog.Exitf("failed to read kubelet config: %v", err)
		}
	case "https":
		restConfig, err := kubeconf.InsecureConfig(u.String(), resourcemonitorArgs.APIAuthTokenFile)
		if err != nil {
			klog.Exitf("failed to initialize rest config for kubelet config uri: %v", err)
		}

		klConfig, err = kubeconf.GetKubeletConfiguration(restConfig)
		if err != nil {
			klog.Exitf("failed to get kubelet config from configz endpoint: %v", err)
		}
	default:
		klog.Exitf("unsupported URI scheme: %v", u.Scheme)
	}

	tmPolicy := string(topologypolicy.DetectTopologyPolicy(klConfig.TopologyManagerPolicy, klConfig.TopologyManagerScope))
	klog.Infof("detected kubelet Topology Manager policy %q", tmPolicy)

	// Get new TopologyUpdater instance
	instance := topology.NewTopologyUpdater(*args, *resourcemonitorArgs, tmPolicy)

	if err = instance.Run(); err != nil {
		klog.Exit(err)
	}
}

func parseArgs(flags *flag.FlagSet, osArgs ...string) (*topology.Args, *resourcemonitor.Args) {
	args, resourcemonitorArgs := initFlags(flags)

	_ = flags.Parse(osArgs)
	if len(flags.Args()) > 0 {
		fmt.Fprintf(flags.Output(), "unknown command line argument: %s\n", flags.Args()[0])
		flags.Usage()
		os.Exit(2)
	}

	if len(resourcemonitorArgs.KubeletConfigURI) == 0 {
		if len(utils.NodeName()) == 0 {
			fmt.Fprintf(flags.Output(), "unable to determine the default kubelet config endpoint 'https://${NODE_NAME}:%d/configz' due to empty NODE_NAME environment, "+
				"please either define the NODE_NAME environment variable or specify endpoint with the -kubelet-config-uri flag\n", kubeletSecurePort)
			os.Exit(1)
		}
		resourcemonitorArgs.KubeletConfigURI = fmt.Sprintf("https://%s:%d/configz", utils.NodeName(), kubeletSecurePort)
	}

	return args, resourcemonitorArgs
}

func initFlags(flagset *flag.FlagSet) (*topology.Args, *resourcemonitor.Args) {
	args := &topology.Args{}
	resourcemonitorArgs := &resourcemonitor.Args{}

	flagset.BoolVar(&args.Oneshot, "oneshot", false,
		"Update once and exit")
	flagset.BoolVar(&args.NoPublish, "no-publish", false,
		"Do not publish discovered features to the cluster-local Kubernetes API server.")
	flagset.StringVar(&args.KubeConfigFile, "kubeconfig", "",
		"Kube config file.")
	flagset.DurationVar(&resourcemonitorArgs.SleepInterval, "sleep-interval", time.Duration(60)*time.Second,
		"Time to sleep between CR updates. Non-positive value implies no CR updatation (i.e. infinite sleep). [Default: 60s]")
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

	klog.InitFlags(flagset)

	return args, resourcemonitorArgs
}
