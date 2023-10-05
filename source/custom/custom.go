/*
Copyright 2020-2021 The Kubernetes Authors.

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

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "custom"

type CustomRule struct {
	nfdv1alpha1.Rule
}

type config []CustomRule

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *config {
	return &config{}
}

// customSource implements the LabelSource and ConfigurableSource interfaces.
type customSource struct {
	config *config
}

// Singleton source instance
var (
	src                           = customSource{config: newDefaultConfig()}
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns the name of the feature source
func (s *customSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *customSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *customSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *customSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *config:
		s.config = v
	default:
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Priority method of the LabelSource interface
func (s *customSource) Priority() int { return 10 }

// GetLabels method of the LabelSource interface
func (s *customSource) GetLabels() (source.FeatureLabels, error) {
	// Get raw features from all sources
	features := source.GetAllFeatures()

	labels := source.FeatureLabels{}
	allFeatureConfig := append(getStaticFeatureConfig(), *s.config...)
	allFeatureConfig = append(allFeatureConfig, getDirectoryFeatureConfig()...)
	klog.V(2).InfoS("resolving custom features", "configuration", utils.DelayedDumper(allFeatureConfig))
	// Iterate over features
	for _, rule := range allFeatureConfig {
		ruleOut, err := rule.Execute(features)
		if err != nil {
			klog.ErrorS(err, "failed to execute rule")
			continue
		}

		for n, v := range ruleOut.Labels {
			labels[n] = v
		}
		// Feed back rule output to features map for subsequent rules to match
		features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Labels)
		features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Vars)
	}

	return labels, nil
}

func init() {
	source.Register(&src)
}
