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

//go:generate go tool mockery --name=LabelSource --inpackage

import (
	"context"
	"fmt"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

// Source is the base interface for all other source interfaces
type Source interface {
	// Name returns a friendly name for this source
	Name() string
}

// FeatureSource is an interface for discovering node features
type FeatureSource interface {
	Source

	// Discover does feature discovery
	Discover() error

	// GetFeatures returns discovered features in raw form
	GetFeatures() *nfdv1alpha1.Features
}

// LabelSource represents a source of node feature labels
type LabelSource interface {
	Source

	// GetLabels returns discovered feature labels
	GetLabels() (FeatureLabels, error)

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

// SupplementalSource represents a source that does not belong to the core set
// sources to be used in production, e.g. is deprecated, very experimental or
// purposed for testing only.
type SupplementalSource interface {
	Source

	// DisableByDefault returns true if the source should be disabled by
	// default in production.
	DisableByDefault() bool
}

// EventSource is an interface for a source that can send events
type EventSource interface {
	FeatureSource

	// SetNotifyChannel sets the notification channel used to send updates about feature changes.
	// The provided channel will receive a notification (a pointer to the FeatureSource) whenever
	// the source detects new or updated features, typically after a successful Discover operation.
	SetNotifyChannel(ctx context.Context, ch chan *FeatureSource) error
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

// Register registers a source.
func Register(s Source) {
	if name, ok := sources[s.Name()]; ok {
		panic(fmt.Sprintf("source %q already registered", name))
	}
	sources[s.Name()] = s
}

// GetFeatureSource returns a registered FeatureSource interface
func GetFeatureSource(name string) FeatureSource {
	if s, ok := sources[name].(FeatureSource); ok {
		return s
	}
	return nil
}

// GetAllFeatureSources returns all registered label sources
func GetAllFeatureSources() map[string]FeatureSource {
	all := make(map[string]FeatureSource)
	for k, v := range sources {
		if s, ok := v.(FeatureSource); ok {
			all[k] = s
		}
	}
	return all
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

// GetAllEventSources returns all registered event sources
func GetAllEventSources() map[string]EventSource {
	all := make(map[string]EventSource)
	for k, v := range sources {
		if s, ok := v.(EventSource); ok {
			all[k] = s
		}
	}
	return all
}

// GetAllFeatures returns a combined set of all features from all feature
// sources.
func GetAllFeatures() *nfdv1alpha1.Features {
	features := nfdv1alpha1.NewFeatures()
	for n, s := range GetAllFeatureSources() {
		f := s.GetFeatures()
		for k, v := range f.Flags {
			// Prefix feature with the name of the source
			k = n + "." + k
			features.Flags[k] = v
		}
		for k, v := range f.Attributes {
			// Prefix feature with the name of the source
			k = n + "." + k
			features.Attributes[k] = v
		}
		for k, v := range f.Instances {
			// Prefix feature with the name of the source
			k = n + "." + k
			features.Instances[k] = v
		}
	}
	return features
}
