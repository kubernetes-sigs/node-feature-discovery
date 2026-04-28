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
	Short: "Test a NodeFeatureRule or NodeFeatureGroup file against a Node",
	Long:  `Test a NodeFeatureRule or NodeFeatureGroup file against a Node to ensure it is valid before applying it to a cluster`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rule == "" {
			return fmt.Errorf("--rule-file must be specified")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Evaluating %s against Node %s\n", rule, node)
		errs := kubectlnfd.Test(rule, node, kubeconfig)
		if len(errs) > 0 {
			fmt.Printf("%s is not valid for Node %s\n", rule, node)
			for _, e := range errs {
				cmd.PrintErrln(e)
			}
			os.Exit(1)
		}
		fmt.Printf("%s is valid for Node %s\n", rule, node)
	},
}

func init() {
	RootCmd.AddCommand(testCmd)

	testCmd.Flags().StringVarP(&rule, "rule-file", "f", "", "Path to the NodeFeatureRule or NodeFeatureGroup file to test")
	testCmd.Flags().StringVarP(&node, "nodename", "n", "", "Node to test against")
	testCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig file to use")
	if err := testCmd.MarkFlagRequired("nodename"); err != nil {
		panic(err)
	}
}
