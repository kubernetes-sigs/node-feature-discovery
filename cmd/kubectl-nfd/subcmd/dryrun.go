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
	Short: "Process a NodeFeatureRule file against a NodeFeature file",
	Long:  `Process a NodeFeatureRule file against a local NodeFeature file to dry run the rule against a node before applying it to a cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Evaluating NodeFeatureRule %q against NodeFeature %q\n", nodefeaturerule, nodefeature)
		err := kubectlnfd.DryRun(nodefeaturerule, nodefeature)
		if len(err) > 0 {
			fmt.Printf("NodeFeatureRule %q is not valid for NodeFeature %q\n", nodefeaturerule, nodefeature)
			for _, e := range err {
				cmd.PrintErrln(e)
			}
			// Return non-zero exit code to indicate failure
			os.Exit(1)
		}
		fmt.Printf("NodeFeatureRule %q is valid for NodeFeature %q\n", nodefeaturerule, nodefeature)
	},
}

func init() {
	RootCmd.AddCommand(dryrunCmd)

	dryrunCmd.Flags().StringVarP(&nodefeaturerule, "nodefeaturerule-file", "f", "", "Path to the NodeFeatureRule file to validate")
	dryrunCmd.Flags().StringVarP(&nodefeature, "nodefeature-file", "n", "", "Path to the NodeFeature file to validate against")
	err := dryrunCmd.MarkFlagRequired("nodefeaturerule-file")
	if err != nil {
		panic(err)
	}
	err = dryrunCmd.MarkFlagRequired("nodefeature-file")
	if err != nil {
		panic(err)
	}
}
