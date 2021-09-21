/*
Copyright 2018-2021 The Kubernetes Authors.

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

package iommu

import (
	"fmt"
	"io/ioutil"

	"sigs.k8s.io/node-feature-discovery/source"
)

const Name = "iommu"

// iommuSource implements the LabelSource interface.
type iommuSource struct{}

func (s *iommuSource) Name() string { return Name }

// Singleton source instance
var (
	src iommuSource
	_   source.LabelSource = &src
)

// Priority method of the LabelSource interface
func (s *iommuSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *iommuSource) GetLabels() (source.FeatureLabels, error) {
	features := source.FeatureLabels{}

	// Check if any iommu devices are available
	devices, err := ioutil.ReadDir(source.SysfsDir.Path("class/iommu/"))
	if err != nil {
		return nil, fmt.Errorf("failed to check for IOMMU support: %v", err)
	}

	if len(devices) > 0 {
		features["enabled"] = true
	}

	return features, nil
}

func init() {
	source.Register(&src)
}
