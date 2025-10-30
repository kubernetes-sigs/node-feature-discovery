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

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test a NodeFeatureRule file against a Node",
	Long:  `Test a NodeFeatureRule file against a Node to ensure it is valid before applying it to a cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Evaluating NodeFeatureRule against Node %s\n", node)
		err := kubectlnfd.Test(nodefeaturerule, node, kubeconfig)
		if len(err) > 0 {
			fmt.Printf("NodeFeatureRule is not valid for Node %s\n", node)
			for _, e := range err {
				cmd.PrintErrln(e)
			}
			// Return non-zero exit code to indicate failure
			os.Exit(1)
		}
		fmt.Printf("NodeFeatureRule is valid for Node %s\n", node)
	},
}

func init() {
	RootCmd.AddCommand(testCmd)

	testCmd.Flags().StringVarP(&nodefeaturerule, "nodefeaturerule-file", "f", "", "Path to the NodeFeatureRule file to validate")
	testCmd.Flags().StringVarP(&node, "nodename", "n", "", "Node to validate against")
	testCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig file to use")
	if err := testCmd.MarkFlagRequired("nodefeaturerule-file"); err != nil {
		panic(err)
	}
	if err := testCmd.MarkFlagRequired("nodename"); err != nil {
		panic(err)
	}
}
