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

package source

import (
	"fmt"
)

// Source is the base interface for all other source interfaces
type Source interface {
	// Name returns a friendly name for this source
	Name() string
}

// LabelSource represents a source of node feature labels
type LabelSource interface {
	Source

	// Discover returns discovered feature labels
	Discover() (FeatureLabels, error)

	// Priority returns the priority of the source
	Priority() int
}

// ConfigurableSource is an interface for a source that can be configured
type ConfigurableSource interface {
	Source

	// NewConfig returns a new default config of the source
	NewConfig() Config

	// GetConfig returns the effective configuration of the source
	GetConfig() Config

	// SetConfig changes the effective configuration of the source
	SetConfig(Config)
}

// TestSource represents a source purposed for testing only
type TestSource interface {
	Source

	// IsTestSource returns true if the source is not for production
	IsTestSource() bool
}

// FeatureLabelValue represents the value of one feature label
type FeatureLabelValue interface{}

// FeatureLabels is a collection of feature labels
type FeatureLabels map[string]FeatureLabelValue

// Config is the generic interface for source configuration data
type Config interface {
}

// sources contain all registered sources
var sources = make(map[string]Source)

// RegisterSource registers a source
func Register(s Source) {
	if name, ok := sources[s.Name()]; ok {
		panic(fmt.Sprintf("source %q already registered", name))
	}
	sources[s.Name()] = s
}

// GetLabelSource a registered label source
func GetLabelSource(name string) LabelSource {
	if s, ok := sources[name].(LabelSource); ok {
		return s
	}
	return nil
}

// GetAllLabelSources returns all registered label sources
func GetAllLabelSources() map[string]LabelSource {
	all := make(map[string]LabelSource)
	for k, v := range sources {
		if s, ok := v.(LabelSource); ok {
			all[k] = s
		}
	}
	return all
}

// GetConfigurableSource a registered configurable source
func GetConfigurableSource(name string) ConfigurableSource {
	if s, ok := sources[name].(ConfigurableSource); ok {
		return s
	}
	return nil
}

// GetAllConfigurableSources returns all registered configurable sources
func GetAllConfigurableSources() map[string]ConfigurableSource {
	all := make(map[string]ConfigurableSource)
	for k, v := range sources {
		if s, ok := v.(ConfigurableSource); ok {
			all[k] = s
		}
	}
	return all
}
