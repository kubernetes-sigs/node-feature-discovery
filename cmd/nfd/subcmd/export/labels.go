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
	"maps"
	"regexp"
	"slices"
	"sort"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	worker "sigs.k8s.io/node-feature-discovery/pkg/nfd-worker"
	"sigs.k8s.io/node-feature-discovery/source"
)

var (
	exportLabelsExample = `
# Export node labels to stdout (prints to terminal)
nfd export features

# Export node labels to a file instead
nfd export features --path /tmp/labels.json`
)

func NewLabelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "labels",
		Short:   "Export feature labels for given node",
		Example: exportLabelsExample,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Determine enabled feature sources
			featureSources := make(map[string]source.FeatureSource)
			for n, s := range source.GetAllFeatureSources() {
				if ts, ok := s.(source.SupplementalSource); !ok || !ts.DisableByDefault() {
					err := s.Discover()
					if err != nil {
						return err
					}
					featureSources[n] = s
				}
			}
			featureSourceList := slices.Collect(maps.Values(featureSources))
			sort.Slice(featureSourceList, func(i, j int) bool { return featureSourceList[i].Name() < featureSourceList[j].Name() })

			// Determine enabled label sources
			labelSources := make(map[string]source.LabelSource)
			for n, s := range source.GetAllLabelSources() {
				if ts, ok := s.(source.SupplementalSource); !ok || !ts.DisableByDefault() {
					labelSources[n] = s
				}
			}
			labelSourcesList := slices.Collect(maps.Values(labelSources))
			sort.Slice(labelSourcesList, func(i, j int) bool {
				iP, jP := labelSourcesList[i].Priority(), labelSourcesList[j].Priority()
				if iP != jP {
					return iP < jP
				}
				return labelSourcesList[i].Name() < labelSourcesList[j].Name()
			})

			labels := worker.Labels{}
			labelWhiteList := *regexp.MustCompile("")

			// Get labels from all enabled label sources
			for _, source := range labelSourcesList {
				labelsFromSource, err := worker.GetFeatureLabels(source, labelWhiteList)
				if err != nil {
					klog.ErrorS(err, "discovery failed", "source", source.Name())
					continue
				}
				maps.Copy(labels, labelsFromSource)
			}

			exportedLabels, err := json.MarshalIndent(labels, "", "    ")
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
	ExportCmd.AddCommand(NewLabelsCmd())
}
