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

package match

import (
	"fmt"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/parser"
	"sigs.k8s.io/node-feature-discovery/source"
)

type GenericMatcher struct {
	SourceName   string
	Initialized  bool
	NodeFeatures *nfdv1alpha1.Features
	NodeLabels   source.FeatureLabels

	Validate ValidateFunc
}

func (m *GenericMatcher) Init() (err error) {
	f := source.GetFeatureSource(m.SourceName)
	if err := f.Discover(); err != nil {
		return err
	}
	m.NodeFeatures = f.GetFeatures()

	m.NodeLabels, err = source.GetLabelSource(m.SourceName).GetLabels()
	if err != nil {
		return err
	}

	m.Initialized = true

	return nil
}

func (m *GenericMatcher) Check(entry parser.EntryGetter) (bool, error) {
	if !m.Initialized {
		if err := m.Init(); err != nil {
			return false, err
		}
	}

	return m.Validate(entry, m.NodeLabels, m.NodeFeatures)
}

func NewGenericMatcher(sourceName string, validate ValidateFunc) *GenericMatcher {
	return &GenericMatcher{
		SourceName: sourceName,
		Validate:   validate,
	}
}

func ValidateDefault(imageLabel parser.EntryGetter, nodeLabels source.FeatureLabels, _ *nfdv1alpha1.Features) (bool, error) {
	if v, ok := nodeLabels[fmt.Sprintf("%s.%s", imageLabel.Feature(), imageLabel.Option())]; ok {
		res, err := imageLabel.ValueEqualTo(v)
		return res, err
	}
	return false, nil
}
