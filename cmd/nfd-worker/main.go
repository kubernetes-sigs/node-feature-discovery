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

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/features"
	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	klogutils "sigs.k8s.io/node-feature-discovery/pkg/utils/klog"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-worker"
)

func main() {
	flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	printVersion := flags.Bool("version", false, "Print version and exit.")

	// Add FeatureGates flag
	if err := features.NFDMutableFeatureGate.Add(features.DefaultNFDFeatureGates); err != nil {
		klog.ErrorS(err, "failed to add default feature gates")
		os.Exit(1)
	}
	features.NFDMutableFeatureGate.AddFlag(flags)

	args := parseArgs(flags, os.Args[1:]...)

	if *printVersion {
		fmt.Println(ProgramName, version.Get())
		os.Exit(0)
	}

	// Assert that the version is known
	if version.Undefined() {
		klog.InfoS("version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always --match 'v*'`\" during build or run.")
	}

	// Get new NfdWorker instance
	instance, err := worker.NewNfdWorker(worker.WithArgs(args))
	if err != nil {
		klog.ErrorS(err, "failed to initialize NfdWorker instance")
		os.Exit(1)
	}

	if err = instance.Run(); err != nil {
		klog.ErrorS(err, "error while running")
		os.Exit(1)
	}
}

func parseArgs(flags *flag.FlagSet, osArgs ...string) *worker.Args {

	// We allow exportFlag to be a boolean or string, and need to check it here
	var exportFlag boolOrStringValue
	args, overrides := initFlags(flags, &exportFlag)

	_ = flags.Parse(osArgs)
	if len(flags.Args()) > 0 {
		fmt.Fprintf(flags.Output(), "unknown command line argument: %s\n", flags.Args()[0])
		flags.Usage()
		os.Exit(2)
	}

	// Was export set?
	if exportFlag.IsSet {

		// Case 1: Only -export is defined, print labels to screen
		if exportFlag.Value == "true" {
			args.Export = true

		} else {
			// Case 2: a specific filename is provided to export to
			args.Export = true
			args.ExportPath = exportFlag.Value
		}
	}

	// Handle overrides
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "no-publish":
			args.Overrides.NoPublish = overrides.NoPublish
		case "feature-sources":
			args.Overrides.FeatureSources = overrides.FeatureSources
		case "label-sources":
			args.Overrides.LabelSources = overrides.LabelSources
		case "no-owner-refs":
			args.Overrides.NoOwnerRefs = overrides.NoOwnerRefs
		}
	})

	return args
}

func initFlags(flagset *flag.FlagSet, exportFlag *boolOrStringValue) (*worker.Args, *worker.ConfigOverrideArgs) {
	args := &worker.Args{}

	flagset.StringVar(&args.ConfigFile, "config", "/etc/kubernetes/node-feature-discovery/nfd-worker.conf",
		"Config file to use.")
	flagset.StringVar(&args.Kubeconfig, "kubeconfig", "",
		"Kubeconfig to use")
	flagset.BoolVar(&args.Oneshot, "oneshot", false,
		"Do not publish feature labels")
	flagset.IntVar(&args.Port, "port", 8080,
		"Port on which to metrics and healthz endpoints are served")
	flagset.StringVar(&args.Options, "options", "",
		"Specify config options from command line. Config options are specified "+
			"in the same format as in the config file (i.e. json or yaml). These options")
	flagset.Var(exportFlag, "export", "Export features from a static, non-Kubernetes node. Can be boolean (-export) or path (-export=<path>).")

	args.Klog = klogutils.InitKlogFlags(flagset)

	// Flags overlapping with config file options
	overrides := &worker.ConfigOverrideArgs{
		FeatureSources: &utils.StringSliceVal{},
		LabelSources:   &utils.StringSliceVal{},
	}
	overrides.NoPublish = flagset.Bool("no-publish", false,
		"Do not publish discovered features, disable connection to nfd-master and don't create NodeFeature object.")
	overrides.NoOwnerRefs = flagset.Bool("no-owner-refs", false,
		"Do not set owner references for NodeFeature object.")
	flagset.Var(overrides.FeatureSources, "feature-sources",
		"Comma separated list of feature sources. Special value 'all' enables all sources. "+
			"Prefix the source name with '-' to disable it.")
	flagset.Var(overrides.LabelSources, "label-sources",
		"Comma separated list of label sources. Special value 'all' enables all sources. "+
			"Prefix the source name with '-' to disable it.")

	return args, overrides
}
