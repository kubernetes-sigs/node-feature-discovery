/*
Copyright 2021 The Kubernetes Authors

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

package crypto

import (
	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/source"
)

type Config struct{}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{}
}

// Implement FeatureSource interface
type Source struct {
	config *Config
}

func (s Source) Name() string { return "crypto" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return s.config }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		klog.Fatalf("invalid config type: %T", conf)
	}
}

func (s *Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Check for Crypto Express Cards
	cards, err := discoverCEX()
	if err != nil {
		klog.Errorf("failed to detect Crypto Express: %v", err)
	} else if cards != nil && len(cards) > 0 {
		features["cex.present"] = true
	}

	return features, nil
}
