/*
Copyright 2024 The Kubernetes Authors.

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

package compat

import (
	"github.com/spf13/cobra"
)

var specFilePath string

var createArtifactCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach compatibility artifact to image",
	Long:  "Attach image compatibility artifact for nodes validation and better pod scheduling",
	RunE: func(cmd *cobra.Command, args []string) error {

		return nil
	},
}

func init() {
	CompatCmd.AddCommand(createArtifactCmd)
	createArtifactCmd.Flags().StringVar(&specFilePath, "spec-file", "", "Path to file with image compatibility spec")
	createArtifactCmd.MarkFlagRequired("spec-file")
}
