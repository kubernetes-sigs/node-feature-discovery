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
	"io/ioutil"
	"strconv"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/source"
)

const Name = "iommu"

const IommuFeature = "iommu"

// iommuSource implements the LabelSource interface.
type iommuSource struct {
	features *feature.DomainFeatures
}

func (s *iommuSource) Name() string { return Name }

// Singleton source instance
var (
	src iommuSource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

// Priority method of the LabelSource interface
func (s *iommuSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *iommuSource) GetLabels() (source.FeatureLabels, error) {
	features := source.FeatureLabels{}

	if s.features.Values[IommuFeature].Elements["enabled"] == "true" {
		features["enabled"] = "true"
	}

	return features, nil
}

// Discover method of the FeatureSource interface
func (s *iommuSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	// Check if any iommu devices are available
	if devices, err := ioutil.ReadDir(source.SysfsDir.Path("class/iommu/")); err != nil {
		klog.Errorf("failed to check for IOMMU support: %v", err)
	} else {
		f := map[string]string{"enabled": strconv.FormatBool(len(devices) > 0)}
		s.features.Values[IommuFeature] = feature.NewValueFeatures(f)
	}

	return nil
}

// GetFeatures method of the FeatureSource Interface.
func (s *iommuSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

func init() {
	source.Register(&src)
}
