/*
Copyright 2017 The Kubernetes Authors.

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

package fake

import "sigs.k8s.io/node-feature-discovery/source"

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "fake" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return nil }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return nil }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(source.Config) {}

// Configure method of the FeatureSource interface
func (s Source) Configure([]byte) error { return nil }

// Discover returns feature names for some fake features.
func (s Source) Discover() (source.Features, error) {
	// Adding three fake features.
	features := source.Features{
		"fakefeature1": true,
		"fakefeature2": true,
		"fakefeature3": true,
	}

	return features, nil
}
