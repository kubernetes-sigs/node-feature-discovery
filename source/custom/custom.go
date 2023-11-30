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
	"os"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
	api "sigs.k8s.io/node-feature-discovery/source/custom/api"
)

// Name of this feature source
const Name = "custom"

// The config files use the internal API type.
type config []api.Rule

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *config {
	return &config{}
}

// customSource implements the LabelSource and ConfigurableSource interfaces.
type customSource struct {
	config *config
	// The rules are stored in the NFD API format that is a superset of our
	// internal API and provides the functions for rule matching.
	rules []nfdv1alpha1.Rule
}

// Singleton source instance
var (
	src = customSource{
		config: &config{},
		rules:  []nfdv1alpha1.Rule{},
	}
	_ source.LabelSource        = &src
	_ source.ConfigurableSource = &src
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
		r := []api.Rule(*v)
		s.rules = convertInternalRulesToNfdApi(&r)
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
	allFeatureConfig := append(getStaticRules(), s.rules...)
	allFeatureConfig = append(allFeatureConfig, getDropinDirRules()...)
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

func convertInternalRulesToNfdApi(in *[]api.Rule) []nfdv1alpha1.Rule {
	out := make([]nfdv1alpha1.Rule, len(*in))
	for i := range *in {
		if err := api.ConvertRuleToV1alpha1(&(*in)[i], &out[i]); err != nil {
			klog.ErrorS(err, "FATAL: API conversion failed")
			os.Exit(255)
		}
	}
	return out
}

func init() {
	source.Register(&src)
}
