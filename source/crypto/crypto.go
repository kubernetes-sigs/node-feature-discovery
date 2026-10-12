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

package crypto

import (
	"fmt"
	"strconv"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "crypto"

// CexCardFeature is the name of the feature set that holds all discovered CEX cards.
const CexCardFeature = "cex-card"

// cryptoSource implements the FeatureSource and LabelSource interfaces.
type cryptoSource struct {
	features *nfdv1alpha1.Features
}

// Singleton source instance
var (
	src                      = cryptoSource{}
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

// Name returns the name of the feature source
func (s *cryptoSource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *cryptoSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *cryptoSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	instances, ok := features.Instances[CexCardFeature]
	if !ok || len(instances.Elements) == 0 {
		return labels, nil
	}

	labels["cex.present"] = true
	labels["cex.count"] = strconv.Itoa(len(instances.Elements))

	cardTypes := make(map[string]bool)
	cardModes := make(map[string]bool)
	for _, card := range instances.Elements {
		if cardType, ok := card.Attributes["type"]; ok {
			cardTypes[cardType] = true
		}
		if mode, ok := card.Attributes["mode"]; ok {
			cardModes[mode] = true
		}
	}

	for cardType := range cardTypes {
		labels["cex.type-"+cardType] = true
	}
	for mode := range cardModes {
		labels["cex.mode-"+mode] = true
	}

	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *cryptoSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	cards, err := detectCexCards()
	if err != nil {
		return fmt.Errorf("failed to detect CEX cards: %w", err)
	}

	if len(cards) > 0 {
		s.features.Instances[CexCardFeature] = nfdv1alpha1.InstanceFeatureSet{Elements: cards}
	}

	klog.V(3).InfoS("discovered features", "featureSource", s.Name(), "features", utils.DelayedDumper(s.features))

	return nil
}

// GetFeatures method of the FeatureSource interface
func (s *cryptoSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

func init() {
	source.Register(&src)
}
