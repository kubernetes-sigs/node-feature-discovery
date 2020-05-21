/*
Copyright 2020 The Kubernetes Authors.

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

package custom

import (
	"fmt"
	"log"

	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/custom/rules"
)

// Custom Features Configurations
type MatchRule struct {
	PciID      *rules.PciIDRule      `json:"pciId,omitempty"`
	UsbID      *rules.UsbIDRule      `json:"usbId,omitempty"`
	LoadedKMod *rules.LoadedKModRule `json:"loadedKMod,omitempty"`
}

type FeatureSpec struct {
	Name    string      `json:"name"`
	MatchOn []MatchRule `json:"matchOn"`
}

type config []FeatureSpec

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *config {
	return &config{}
}

// Implements FeatureSource Interface
type Source struct {
	config *config
}

// Return name of the feature source
func (s Source) Name() string { return "custom" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return s.config }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *config:
		s.config = v
	default:
		log.Printf("PANIC: invalid config type: %T", conf)
	}
}

// Discover features
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}
	allFeatureConfig := append(getStaticFeatureConfig(), *s.config...)
	log.Printf("INFO: Custom features: %+v", allFeatureConfig)
	// Iterate over features
	for _, customFeature := range allFeatureConfig {
		featureExist, err := s.discoverFeature(customFeature)
		if err != nil {
			return features, fmt.Errorf("failed to discover feature: %s. %s", customFeature.Name, err.Error())
		}
		if featureExist {
			features[customFeature.Name] = true
		}
	}
	return features, nil
}

// Process a single feature by Matching on the defined rules.
// A feature is present if all defined Rules in a MatchRule return a match.
func (s Source) discoverFeature(feature FeatureSpec) (bool, error) {
	for _, rule := range feature.MatchOn {
		// PCI ID rule
		if rule.PciID != nil {
			match, err := rule.PciID.Match()
			if err != nil {
				return false, err
			}
			if !match {
				continue
			}
		}
		// USB ID rule
		if rule.UsbID != nil {
			match, err := rule.UsbID.Match()
			if err != nil {
				return false, err
			}
			if !match {
				continue
			}
		}
		// Loaded kernel module rule
		if rule.LoadedKMod != nil {
			match, err := rule.LoadedKMod.Match()
			if err != nil {
				return false, err
			}
			if !match {
				continue
			}
		}
		return true, nil
	}
	return false, nil
}
