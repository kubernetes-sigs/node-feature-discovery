/*
Copyright 2025 The Kubernetes Authors.

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

package export

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"sigs.k8s.io/node-feature-discovery/source"
)

var (
	exportFeaturesExample = `
# Export node features to stdout (prints to terminal)
nfd export features

# Export node features to a file instead
nfd export features --path /tmp/features.json`
)

func NewExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "features",
		Short:   "Export features for given node",
		Example: exportFeaturesExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			sources := map[string]source.FeatureSource{}
			for k, v := range source.GetAllFeatureSources() {
				if ts, ok := v.(source.SupplementalSource); ok && ts.DisableByDefault() {
					continue
				}
				sources[k] = v
			}

			// Discover all feature sources
			for _, s := range sources {
				if err := s.Discover(); err != nil {
					return fmt.Errorf("error during discovery of source %s: %w", s.Name(), err)
				}
			}

			features := source.GetAllFeatures()
			exportedLabels, err := json.MarshalIndent(features, "", "    ")
			if err != nil {
				return err
			}

			if outputPath != "" {
				err = writeToFile(outputPath, string(exportedLabels))
			} else {
				fmt.Println(string(exportedLabels))
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "path", "p", "", "export to this JSON path")
	return cmd
}

func init() {
	ExportCmd.AddCommand(NewExportCmd())
}
