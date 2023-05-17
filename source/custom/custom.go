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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/custom/rules"
)

// Name of this feature source
const Name = "custom"

// LegacyMatcher contains the legacy custom rules.
type LegacyMatcher struct {
	PciID      *rules.PciIDRule      `json:"pciId,omitempty"`
	UsbID      *rules.UsbIDRule      `json:"usbId,omitempty"`
	LoadedKMod *rules.LoadedKModRule `json:"loadedKMod,omitempty"`
	CpuID      *rules.CpuIDRule      `json:"cpuId,omitempty"`
	Kconfig    *rules.KconfigRule    `json:"kConfig,omitempty"`
	Nodename   *rules.NodenameRule   `json:"nodename,omitempty"`
}

type LegacyRule struct {
	Name    string          `json:"name"`
	Value   *string         `json:"value,omitempty"`
	MatchOn []LegacyMatcher `json:"matchOn"`
}

type Rule struct {
	nfdv1alpha1.Rule
}

type config []CustomRule

type CustomRule struct {
	*LegacyRule
	*Rule
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *config {
	return &config{}
}

// customSource implements the LabelSource and ConfigurableSource interfaces.
type customSource struct {
	config *config
}

type legacyRule interface {
	Match() (bool, error)
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
		ruleOut, err := rule.execute(features)
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

func (r *CustomRule) execute(features *nfdv1alpha1.Features) (nfdv1alpha1.RuleOutput, error) {
	if r.LegacyRule != nil {
		ruleOut, err := r.LegacyRule.execute()
		if err != nil {
			return nfdv1alpha1.RuleOutput{}, fmt.Errorf("failed to execute legacy rule %s: %w", r.LegacyRule.Name, err)
		}
		return nfdv1alpha1.RuleOutput{Labels: ruleOut}, nil
	}

	if r.Rule != nil {
		ruleOut, err := r.Rule.Execute(features)
		if err != nil {
			return ruleOut, fmt.Errorf("failed to execute rule %s: %w", r.Rule.Name, err)
		}
		return ruleOut, nil
	}

	return nfdv1alpha1.RuleOutput{}, fmt.Errorf("BUG: an empty rule, this really should not happen")
}

func (r *LegacyRule) execute() (map[string]string, error) {
	if len(r.MatchOn) > 0 {
		// Logical OR over the legacy rules
		matched := false
		for _, matcher := range r.MatchOn {
			if m, err := matcher.match(); err != nil {
				return nil, err
			} else if m {
				matched = true
				break
			}
		}
		if !matched {
			return nil, nil
		}
	}

	// Prefix non-namespaced labels with "custom-"
	name := r.Name
	if !strings.Contains(name, "/") {
		name = "custom-" + name
	}

	value := "true"
	if r.Value != nil {
		value = *r.Value
	}

	return map[string]string{name: value}, nil
}

func (m *LegacyMatcher) match() (bool, error) {
	allRules := []legacyRule{
		m.PciID,
		m.UsbID,
		m.LoadedKMod,
		m.CpuID,
		m.Kconfig,
		m.Nodename,
	}

	// return true, nil if all rules match
	matchRules := func(rules []legacyRule) (bool, error) {
		for _, rule := range rules {
			if reflect.ValueOf(rule).IsNil() {
				continue
			}
			if match, err := rule.Match(); err != nil {
				return false, err
			} else if !match {
				return false, nil
			}
		}
		return true, nil
	}

	return matchRules(allRules)
}

// UnmarshalJSON implements the Unmarshaler interface from "encoding/json"
func (c *CustomRule) UnmarshalJSON(data []byte) error {
	// Do a raw parse to determine if this is a legacy rule
	raw := map[string]json.RawMessage{}
	err := yaml.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	for k := range raw {
		if strings.ToLower(k) == "matchon" {
			return yaml.Unmarshal(data, &c.LegacyRule)
		}
	}

	return yaml.Unmarshal(data, &c.Rule)
}

// MarshalJSON implements the Marshaler interface from "encoding/json"
func (c *CustomRule) MarshalJSON() ([]byte, error) {
	if c.LegacyRule != nil {
		return json.Marshal(c.LegacyRule)
	}
	return json.Marshal(c.Rule)
}

func init() {
	source.Register(&src)
}
