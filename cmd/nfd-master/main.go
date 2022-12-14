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
	"regexp"

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

	args := initFlags(flags)
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
			klog.Warningf("-featurerules-controller is deprecated, use '-crd-controller' flag instead")
		}
	})

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

	// Get new NfdMaster instance
	instance, err := master.NewNfdMaster(args)
	if err != nil {
		klog.Exitf("failed to initialize NfdMaster instance: %v", err)
	}

	if err = instance.Run(); err != nil {
		klog.Exit(err)
	}
}

func initFlags(flagset *flag.FlagSet) *master.Args {
	args := &master.Args{
		LabelWhiteList: utils.RegexpVal{Regexp: *regexp.MustCompile("")},
	}

	flagset.StringVar(&args.CaFile, "ca-file", "",
		"Root certificate for verifying connections")
	flagset.StringVar(&args.CertFile, "cert-file", "",
		"Certificate used for authenticating connections")
	flagset.Var(&args.ExtraLabelNs, "extra-label-ns",
		"Comma separated list of allowed extra label namespaces")
	flagset.StringVar(&args.Instance, "instance", "",
		"Instance name. Used to separate annotation namespaces for multiple parallel deployments.")
	flagset.StringVar(&args.KeyFile, "key-file", "",
		"Private key matching -cert-file")
	flagset.StringVar(&args.Kubeconfig, "kubeconfig", "",
		"Kubeconfig to use")
	flagset.Var(&args.LabelWhiteList, "label-whitelist",
		"Regular expression to filter label names to publish to the Kubernetes API server. "+
			"NB: the label namespace is omitted i.e. the filter is only applied to the name part after '/'.")
	flagset.BoolVar(&args.EnableNodeFeatureApi, "enable-nodefeature-api", false,
		"Enable the NodeFeature CRD API for receiving node features. This will automatically disable the gRPC communication.")
	flagset.BoolVar(&args.NoPublish, "no-publish", false,
		"Do not publish feature labels")
	flagset.BoolVar(&args.EnableTaints, "enable-taints", false,
		"Enable node tainting feature")
	flagset.BoolVar(&args.CrdController, "featurerules-controller", true,
		"Enable NFD CRD API controller. DEPRECATED: use -crd-controller instead")
	flagset.BoolVar(&args.CrdController, "crd-controller", true,
		"Enable NFD CRD API controller for processing NodeFeature and NodeFeatureRule objects.")
	flagset.IntVar(&args.Port, "port", 8080,
		"Port on which to listen for connections.")
	flagset.BoolVar(&args.Prune, "prune", false,
		"Prune all NFD related attributes from all nodes of the cluaster and exit.")
	flagset.Var(&args.ResourceLabels, "resource-labels",
		"Comma separated list of labels to be exposed as extended resources.")
	flagset.BoolVar(&args.VerifyNodeName, "verify-node-name", false,
		"Verify worker node name against the worker's TLS certificate. "+
			"Only takes effect when TLS authentication has been enabled.")

	return args
}
