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

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a NodeFeatureRule file",
	Long:  `Validate a NodeFeatureRule file to ensure it is valid before applying it to a cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Validating NodeFeatureRule %s\n", nodefeaturerule)
		err := kubectlnfd.ValidateNFR(nodefeaturerule)
		if len(err) > 0 {
			fmt.Printf("NodeFeatureRule %s is not valid\n", nodefeaturerule)
			for _, e := range err {
				cmd.PrintErrln(e)
			}
			// Return non-zero exit code to indicate failure
			os.Exit(1)
		}
		fmt.Printf("NodeFeatureRule %s is valid\n", nodefeaturerule)
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)

	validateCmd.Flags().StringVarP(&nodefeaturerule, "nodefeaturerule-file", "f", "", "Path to the NodeFeatureRule file to validate")
	err := validateCmd.MarkFlagRequired("nodefeaturerule-file")
	if err != nil {
		panic(err)
	}
}
