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
	kubectlnfd "sigs.k8s.io/node-feature-discovery/pkg/kubectl-nfd"
)

var dryrunCmd = &cobra.Command{
	Use:   "dryrun",
	Short: "Process a NodeFeatureRule or NodeFeatureGroup file against a NodeFeature file",
	Long:  `Process a NodeFeatureRule or NodeFeatureGroup file against a local NodeFeature file to dry run the rule against a node before applying it to a cluster`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rule == "" {
			return fmt.Errorf("--rule-file must be specified")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Evaluating %q against NodeFeature %q\n", rule, nodefeature)
		errs := kubectlnfd.DryRun(rule, nodefeature)
		if len(errs) > 0 {
			fmt.Printf("%q is not valid for NodeFeature %q\n", rule, nodefeature)
			for _, e := range errs {
				cmd.PrintErrln(e)
			}
			os.Exit(1)
		}
		fmt.Printf("%q is valid for NodeFeature %q\n", rule, nodefeature)
	},
}

func init() {
	RootCmd.AddCommand(dryrunCmd)

	dryrunCmd.Flags().StringVarP(&rule, "rule-file", "f", "", "Path to the NodeFeatureRule or NodeFeatureGroup file to dry run")
	dryrunCmd.Flags().StringVarP(&nodefeature, "nodefeature-file", "n", "", "Path to the NodeFeature file to dry run against")
	if err := dryrunCmd.MarkFlagRequired("nodefeature-file"); err != nil {
		panic(err)
	}
}
