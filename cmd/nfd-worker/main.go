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
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/nfd-client/worker"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-worker"
)

func main() {
	flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	printVersion := flags.Bool("version", false, "Print version and exit.")

	args := parseArgs(flags, os.Args[1:]...)

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

	// Get new NfdWorker instance
	instance, err := worker.NewNfdWorker(args)
	if err != nil {
		klog.Exitf("failed to initialize NfdWorker instance: %v", err)
	}

	if err = instance.Run(); err != nil {
		klog.Exit(err)
	}
}

func parseArgs(flags *flag.FlagSet, osArgs ...string) *worker.Args {
	args, overrides := initFlags(flags)

	_ = flags.Parse(osArgs)
	if len(flags.Args()) > 0 {
		fmt.Fprintf(flags.Output(), "unknown command line argument: %s\n", flags.Args()[0])
		flags.Usage()
		os.Exit(2)
	}

	// Handle overrides
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "no-publish":
			args.Overrides.NoPublish = overrides.NoPublish
		case "label-whitelist":
			klog.Warningf("--label-whitelist is deprecated, use 'core.labelWhiteList' option in the config file, instead")
			args.Overrides.LabelWhiteList = overrides.LabelWhiteList
		case "sleep-interval":
			klog.Warningf("--sleep-interval is deprecated, use 'core.sleepInterval' option in the config file, instead")
			args.Overrides.SleepInterval = overrides.SleepInterval
		case "sources":
			klog.Warningf("--sources is deprecated, use 'core.sources' option in the config file, instead")
			args.Overrides.Sources = overrides.Sources
		}
	})

	return args
}

func initFlags(flagset *flag.FlagSet) (*worker.Args, *worker.ConfigOverrideArgs) {
	args := &worker.Args{}

	flagset.StringVar(&args.CaFile, "ca-file", "",
		"Root certificate for verifying connections")
	flagset.StringVar(&args.CertFile, "cert-file", "",
		"Certificate used for authenticating connections")
	flagset.StringVar(&args.ConfigFile, "config", "/etc/kubernetes/node-feature-discovery/nfd-worker.conf",
		"Config file to use.")
	flagset.StringVar(&args.KeyFile, "key-file", "",
		"Private key matching -cert-file")
	flagset.BoolVar(&args.Oneshot, "oneshot", false,
		"Do not publish feature labels")
	flagset.StringVar(&args.Options, "options", "",
		"Specify config options from command line. Config options are specified "+
			"in the same format as in the config file (i.e. json or yaml). These options")
	flagset.StringVar(&args.Server, "server", "localhost:8080",
		"NFD server address to connecto to.")
	flagset.StringVar(&args.ServerNameOverride, "server-name-override", "",
		"Hostname expected from server certificate, useful in testing")

	initKlogFlags(flagset, args)

	// Flags overlapping with config file options
	overrides := &worker.ConfigOverrideArgs{
		LabelWhiteList: &utils.RegexpVal{},
		Sources:        &utils.StringSliceVal{},
	}
	overrides.NoPublish = flagset.Bool("no-publish", false,
		"Do not publish discovered features, disable connection to nfd-master.")
	flagset.Var(overrides.LabelWhiteList, "label-whitelist",
		"Regular expression to filter label names to publish to the Kubernetes API server. "+
			"NB: the label namespace is omitted i.e. the filter is only applied to the name part after '/'. "+
			"DEPRECATED: This parameter should be set via the config file.")
	overrides.SleepInterval = flagset.Duration("sleep-interval", 0,
		"Time to sleep between re-labeling. Non-positive value implies no re-labeling (i.e. infinite sleep). "+
			"DEPRECATED: This parameter should be set via the config file")
	flagset.Var(overrides.Sources, "sources",
		"Comma separated list of feature sources. Special value 'all' enables all feature sources. "+
			"DEPRECATED: This parameter should be set via the config file")

	return args, overrides
}

func initKlogFlags(flagset *flag.FlagSet, args *worker.Args) {
	args.Klog = make(map[string]*utils.KlogFlagVal)

	flags := flag.NewFlagSet("klog flags", flag.ContinueOnError)
	//flags.SetOutput(ioutil.Discard)
	klog.InitFlags(flags)
	flags.VisitAll(func(f *flag.Flag) {
		name := klogConfigOptName(f.Name)
		args.Klog[name] = utils.NewKlogFlagVal(f)
		flagset.Var(args.Klog[name], f.Name, f.Usage)
	})
}

func klogConfigOptName(flagName string) string {
	split := strings.Split(flagName, "_")
	for i, v := range split[1:] {
		split[i+1] = strings.Title(v)
	}
	return strings.Join(split, "")
}
