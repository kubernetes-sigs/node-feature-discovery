/*
Copyright 2017-2021 The Kubernetes Authors.

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

import (
	"fmt"

	"sigs.k8s.io/node-feature-discovery/source"
)

const Name = "fake"

// Configuration file options
type Config struct {
	Labels map[string]string `json:"labels"`
}

// newDefaultConfig returns a new config with defaults values
func newDefaultConfig() *Config {
	return &Config{
		Labels: map[string]string{
			"fakefeature1": "true",
			"fakefeature2": "true",
			"fakefeature3": "true",
		},
	}
}

// fakeSource implements the LabelSource and ConfigurableSource interfaces.
type fakeSource struct {
	config *Config
}

// Singleton source instance
var (
	src fakeSource
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns an identifier string for this feature source.
func (s *fakeSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *fakeSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *fakeSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *fakeSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Configure method of the LabelSource interface
func (s *fakeSource) Configure([]byte) error { return nil }

// Priority method of the LabelSource interface
func (s *fakeSource) Priority() int { return 0 }

// Discover returns feature names for some fake features.
func (s *fakeSource) Discover() (source.FeatureLabels, error) {
	// Adding three fake features.
	features := make(source.FeatureLabels, len(s.config.Labels))
	for k, v := range s.config.Labels {
		features[k] = v
	}

	return features, nil
}

// IsTestSource method of the LabelSource interface
func (s *fakeSource) IsTestSource() bool { return true }

func init() {
	source.Register(&src)
}
