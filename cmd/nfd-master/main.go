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
	"regexp"
	"strconv"
	"strings"

	"github.com/docopt/docopt-go"
	master "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	// ProgramName is the canonical name of this program
	ProgramName = "nfd-master"
)

func main() {
	// Assert that the version is known
	if version.Undefined() {
		log.Print("WARNING: version not set! Set -ldflags \"-X sigs.k8s.io/node-feature-discovery/pkg/version.version=`git describe --tags --dirty --always`\" during build or run.")
	}

	// Parse command-line arguments.
	args, err := argsParse(nil)
	if err != nil {
		log.Fatalf("failed to parse command line: %v", err)
	}

	// Get new NfdMaster instance
	instance, err := master.NewNfdMaster(args)
	if err != nil {
		log.Fatalf("Failed to initialize NfdMaster instance: %v", err)
	}

	if err = instance.Run(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}

// argsParse parses the command line arguments passed to the program.
// The argument argv is passed only for testing purposes.
func argsParse(argv []string) (master.Args, error) {
	args := master.Args{}
	usage := fmt.Sprintf(`%s.

  Usage:
  %s [--no-publish] [--label-whitelist=<pattern>] [--port=<port>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
     [--verify-node-name] [--extra-label-ns=<list>] [--resource-labels=<list>]
  %s -h | --help
  %s --version

  Options:
  -h --help                       Show this screen.
  --version                       Output version and exit.
  --port=<port>                   Port on which to listen for connections.
                                  [Default: 8080]
  --ca-file=<path>                Root certificate for verifying connections
                                  [Default: ]
  --cert-file=<path>              Certificate used for authenticating connections
                                  [Default: ]
  --key-file=<path>               Private key matching --cert-file
                                  [Default: ]
  --verify-node-name              Verify worker node name against CN from the TLS
                                  certificate. Only has effect when TLS authentication
                                  has been enabled.
  --no-publish                    Do not publish feature labels
  --label-whitelist=<pattern>     Regular expression to filter label names to
                                  publish to the Kubernetes API server.
                                  NB: the label namespace is omitted i.e. the filter
                                  is only applied to the name part after '/'.
                                  [Default: ]
  --extra-label-ns=<list>         Comma separated list of allowed extra label namespaces
                                  [Default: ]
  --resource-labels=<list>        Comma separated list of labels to be exposed as extended resources.
                                  [Default: ]`,
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
	args.Port, err = strconv.Atoi(arguments["--port"].(string))
	if err != nil {
		return args, fmt.Errorf("invalid --port defined: %s", err)
	}
	args.LabelWhiteList, err = regexp.Compile(arguments["--label-whitelist"].(string))
	if err != nil {
		return args, fmt.Errorf("error parsing whitelist regex (%s): %s", arguments["--label-whitelist"], err)
	}
	args.VerifyNodeName = arguments["--verify-node-name"].(bool)
	args.ExtraLabelNs = strings.Split(arguments["--extra-label-ns"].(string), ",")
	args.ResourceLabels = strings.Split(arguments["--resource-labels"].(string), ",")

	return args, nil
}
