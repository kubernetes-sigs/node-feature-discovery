/*
Copyright 2023 The Kubernetes Authors.

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

package subcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Path to the NodeFeatureRule file to validate
	nodefeaturerule string
	// Path to the NodeFeature file to run against the NodeFeatureRule
	nodefeature string
	// Node to validate against
	node string
	// kubeconfig file to use
	kubeconfig string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "kubectl-nfd",
	Short: "NFD kubectl plugin",
	Long: `kubectl plugin for NFD
	Debug tool to validate/dryrun/test NodeFeatureRules
	for more information see: 
	https://kubernetes-sigs.github.io/node-feature-discovery/v0.14/usage/customization-guide.html#nodefeaturerule-custom-resource`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
