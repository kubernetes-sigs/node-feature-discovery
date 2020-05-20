/*
Copyright 2019 The Kubernetes Authors.

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
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-worker"
)

func main() {
	// Assert that the version is known
	if version.Undefined() {
		log.Printf("WARNING: version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}

	// Parse command-line arguments.
	args, err := argsParse(nil)
	if err != nil {
		log.Fatalf("failed to parse command line: %v", err)
	}

	// Get new NfdWorker instance
	instance, err := worker.NewNfdWorker(args)
	if err != nil {
		log.Fatalf("Failed to initialize NfdWorker instance: %v", err)
	}

	if err = instance.Run(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}

// argsParse parses the command line arguments passed to the program.
// The argument argv is passed only for testing purposes.
func argsParse(argv []string) (worker.Args, error) {
	args := worker.Args{}
	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish] [--sources=<sources>] [--label-whitelist=<pattern>]
     [--oneshot | --sleep-interval=<seconds>] [--config=<path>]
     [--options=<config>] [--server=<server>] [--server-name-override=<name>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
  %s -h | --help
  %s --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --config=<path>             Config file to use.
                              [Default: /etc/kubernetes/node-feature-discovery/nfd-worker.conf]
  --options=<config>          Specify config options from command line. Config
                              options are specified in the same format as in the
                              config file (i.e. json or yaml). These options
                              will override settings read from the config file.
                              [Default: ]
  --ca-file=<path>            Root certificate for verifying connections
                              [Default: ]
  --cert-file=<path>          Certificate used for authenticating connections
                              [Default: ]
  --key-file=<path>           Private key matching --cert-file
                              [Default: ]
  --server=<server>           NFD server address to connecto to.
                              [Default: localhost:8080]
  --server-name-override=<name> Name (CN) expect from server certificate, useful
                              in testing
                              [Default: ]
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpu,custom,iommu,kernel,local,memory,network,pci,storage,system,usb]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server.
                              NB: the label namespace is omitted i.e. the filter
                              is only applied to the name part after '/'.
                              [Default: ]
  --oneshot                   Label once and exit.
  --sleep-interval=<seconds>  Time to sleep between re-labeling. Non-positive
                              value implies no re-labeling (i.e. infinite
                              sleep). [Default: 60s]`,
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
	args.ConfigFile = arguments["--config"].(string)
	args.KeyFile = arguments["--key-file"].(string)
	args.NoPublish = arguments["--no-publish"].(bool)
	args.Options = arguments["--options"].(string)
	args.Server = arguments["--server"].(string)
	args.ServerNameOverride = arguments["--server-name-override"].(string)
	args.Sources = strings.Split(arguments["--sources"].(string), ",")
	args.LabelWhiteList = arguments["--label-whitelist"].(string)
	args.Oneshot = arguments["--oneshot"].(bool)
	args.SleepInterval, err = time.ParseDuration(arguments["--sleep-interval"].(string))
	if err != nil {
		return args, fmt.Errorf("invalid --sleep-interval specified: %s", err.Error())
	}
	return args, nil
}
