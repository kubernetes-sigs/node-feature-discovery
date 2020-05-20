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
	PciId      *rules.PciIdRule      `json:"pciId,omitempty""`
	UsbId      *rules.UsbIdRule      `json:"usbId,omitempty""`
	LoadedKMod *rules.LoadedKModRule `json:"loadedKMod,omitempty""`
}

type CustomFeature struct {
	Name    string      `json:"name"`
	MatchOn []MatchRule `json:"matchOn"`
}

type NFDConfig []CustomFeature

var Config = NFDConfig{}

// Implements FeatureSource Interface
type Source struct{}

// Return name of the feature source
func (s Source) Name() string { return "custom" }

// Discover features
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}
	allFeatureConfig := append(getStaticFeatureConfig(), Config...)
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
func (s Source) discoverFeature(feature CustomFeature) (bool, error) {
	for _, rule := range feature.MatchOn {
		// PCI ID rule
		if rule.PciId != nil {
			match, err := rule.PciId.Match()
			if err != nil {
				return false, err
			}
			if !match {
				continue
			}
		}
		// USB ID rule
		if rule.UsbId != nil {
			match, err := rule.UsbId.Match()
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
