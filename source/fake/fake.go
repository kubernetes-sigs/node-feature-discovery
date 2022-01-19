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

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "fake"

const FlagFeature = "flag"
const AttributeFeature = "attribute"
const InstanceFeature = "instance"

// Configuration file options
type Config struct {
	Labels            map[string]string `json:"labels"`
	FlagFeatures      []string          `json:"flagFeatures"`
	AttributeFeatures map[string]string `json:"attributeFeatures"`
	InstanceFeatures  []FakeInstance    `json:"instanceFeatures"`
}

type FakeInstance map[string]string

// newDefaultConfig returns a new config with defaults values
func newDefaultConfig() *Config {
	return &Config{
		Labels: map[string]string{
			"fakefeature1": "true",
			"fakefeature2": "true",
			"fakefeature3": "true",
		},
		FlagFeatures: []string{"flag_1", "flag_2", "flag_3"},
		AttributeFeatures: map[string]string{
			"attr_1": "true",
			"attr_2": "false",
			"attr_3": "10",
		},
		InstanceFeatures: []FakeInstance{
			FakeInstance{
				"name":   "instance_1",
				"attr_1": "true",
				"attr_2": "false",
				"attr_3": "10",
				"attr_4": "foobar",
			},
			FakeInstance{
				"name":   "instance_2",
				"attr_1": "true",
				"attr_2": "true",
				"attr_3": "100",
			},
			FakeInstance{
				"name": "instance_3",
			},
		},
	}
}

// fakeSource implements the FeatureSource, LabelSource and ConfigurableSource interfaces.
type fakeSource struct {
	config   *Config
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src fakeSource
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns an identifier string for this feature source.
func (s *fakeSource) Name() string { return Name }

// NewConfig method of the ConfigurableSource interface
func (s *fakeSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the ConfigurableSource interface
func (s *fakeSource) GetConfig() source.Config { return s.config }

// SetConfig method of the ConfigurableSource interface
func (s *fakeSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Discover method of the FeatureSource interface
func (s *fakeSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	s.features.Keys[AttributeFeature] = feature.NewKeyFeatures(s.config.FlagFeatures...)
	s.features.Values[AttributeFeature] = feature.NewValueFeatures(s.config.AttributeFeatures)

	instances := make([]feature.InstanceFeature, len(s.config.InstanceFeatures))
	for i, instanceAttributes := range s.config.InstanceFeatures {
		instances[i] = *feature.NewInstanceFeature(instanceAttributes)
	}
	s.features.Instances[InstanceFeature] = feature.NewInstanceFeatures(instances)

	utils.KlogDump(3, "discovered fake features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface.
func (s *fakeSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

// Priority method of the LabelSource interface
func (s *fakeSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *fakeSource) GetLabels() (source.FeatureLabels, error) {
	labels := make(source.FeatureLabels, len(s.config.Labels))

	for k, v := range s.config.Labels {
		labels[k] = v
	}

	return labels, nil
}

// DisableByDefault method of the SupplementalSource interface.
func (s *fakeSource) DisableByDefault() bool { return true }

func init() {
	source.Register(&src)
}
