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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"

	master "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-master"
)

func main() {
	flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	printVersion := flags.Bool("version", false, "Print version and exit.")

	args, overrides := initFlags(flags)
	// Inject klog flags
	klog.InitFlags(flags)

	_ = flags.Parse(os.Args[1:])
	if len(flags.Args()) > 0 {
		fmt.Fprintf(flags.Output(), "unknown command line argument: %s\n", flags.Args()[0])
		flags.Usage()
		os.Exit(2)
	}

	// Check deprecated flags
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "featurerules-controller":
			klog.InfoS("-featurerules-controller is deprecated, use '-crd-controller' flag instead")
		case "extra-label-ns":
			args.Overrides.ExtraLabelNs = overrides.ExtraLabelNs
		case "deny-label-ns":
			args.Overrides.DenyLabelNs = overrides.DenyLabelNs
		case "label-whitelist":
			args.Overrides.LabelWhiteList = overrides.LabelWhiteList
		case "resource-labels":
			klog.InfoS("-resource-labels is deprecated, extended resources should be managed with NodeFeatureRule objects")
			args.Overrides.ResourceLabels = overrides.ResourceLabels
		case "enable-taints":
			args.Overrides.EnableTaints = overrides.EnableTaints
		case "no-publish":
			args.Overrides.NoPublish = overrides.NoPublish
		case "resync-period":
			args.Overrides.ResyncPeriod = overrides.ResyncPeriod
		}
	})

	if *printVersion {
		fmt.Println(ProgramName, version.Get())
		os.Exit(0)
	}

	// Assert that the version is known
	if version.Undefined() {
		klog.InfoS("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}

	// Plug klog into grpc logging infrastructure
	utils.ConfigureGrpcKlog()

	// Get new NfdMaster instance
	instance, err := master.NewNfdMaster(args)
	if err != nil {
		klog.ErrorS(err, "failed to initialize NfdMaster instance")
		os.Exit(1)
	}

	if err = instance.Run(); err != nil {
		klog.ErrorS(err, "error while running")
		os.Exit(1)
	}
}

func initFlags(flagset *flag.FlagSet) (*master.Args, *master.ConfigOverrideArgs) {
	args := &master.Args{}

	flagset.StringVar(&args.CaFile, "ca-file", "",
		"Root certificate for verifying connections")
	flagset.StringVar(&args.CertFile, "cert-file", "",
		"Certificate used for authenticating connections")
	flagset.StringVar(&args.Instance, "instance", "",
		"Instance name. Used to separate annotation namespaces for multiple parallel deployments.")
	flagset.StringVar(&args.KeyFile, "key-file", "",
		"Private key matching -cert-file")
	flagset.StringVar(&args.ConfigFile, "config", "/etc/kubernetes/node-feature-discovery/nfd-master.conf",
		"Config file to use.")
	flagset.StringVar(&args.Kubeconfig, "kubeconfig", "",
		"Kubeconfig to use")
	flagset.BoolVar(&args.EnableNodeFeatureApi, "enable-nodefeature-api", false,
		"Enable the NodeFeature CRD API for receiving node features. This will automatically disable the gRPC communication.")
	flagset.BoolVar(&args.CrdController, "featurerules-controller", true,
		"Enable NFD CRD API controller. DEPRECATED: use -crd-controller instead")
	flagset.BoolVar(&args.CrdController, "crd-controller", true,
		"Enable NFD CRD API controller for processing NodeFeature and NodeFeatureRule objects.")
	flagset.IntVar(&args.Port, "port", 8080,
		"Port on which to listen for connections.")
	flagset.BoolVar(&args.Prune, "prune", false,
		"Prune all NFD related attributes from all nodes of the cluaster and exit.")
	flagset.BoolVar(&args.VerifyNodeName, "verify-node-name", false,
		"Verify worker node name against the worker's TLS certificate. "+
			"Only takes effect when TLS authentication has been enabled.")
	flagset.StringVar(&args.Options, "options", "",
		"Specify config options from command line. Config options are specified "+
			"in the same format as in the config file (i.e. json or yaml). These options")
	flagset.BoolVar(&args.EnableLeaderElection, "enable-leader-election", false,
		"Enables a leader election. Enable this when running more than one replica on nfd master.")

	overrides := &master.ConfigOverrideArgs{
		LabelWhiteList: &utils.RegexpVal{},
		DenyLabelNs:    &utils.StringSetVal{},
		ExtraLabelNs:   &utils.StringSetVal{},
		ResourceLabels: &utils.StringSetVal{},
		ResyncPeriod:   &utils.DurationVal{Duration: time.Duration(1) * time.Hour},
	}
	flagset.Var(overrides.ExtraLabelNs, "extra-label-ns",
		"Comma separated list of allowed extra label namespaces")
	flagset.Var(overrides.LabelWhiteList, "label-whitelist",
		"Regular expression to filter label names to publish to the Kubernetes API server. "+
			"NB: the label namespace is omitted i.e. the filter is only applied to the name part after '/'.")
	overrides.EnableTaints = flagset.Bool("enable-taints", false,
		"Enable node tainting feature")
	overrides.NoPublish = flagset.Bool("no-publish", false,
		"Do not publish feature labels")
	flagset.Var(overrides.DenyLabelNs, "deny-label-ns",
		"Comma separated list of denied label namespaces")
	flagset.Var(overrides.ResourceLabels, "resource-labels",
		"Comma separated list of labels to be exposed as extended resources. DEPRECATED: use NodeFeatureRule objects instead")
	flagset.Var(overrides.ResyncPeriod, "resync-period",
		"Specify the NFD API controller resync period."+
			"It has an effect when the NodeFeature API has been enabled (with -enable-nodefeature-api).")
	return args, overrides
}
