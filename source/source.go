/*
Copyright 2017 The Kubernetes Authors.

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

// FeatureLabelValue represents the value of one feature label
type FeatureLabelValue interface{}

// FeatureLabels is a collection of feature labels
type FeatureLabels map[string]FeatureLabelValue

// LabelSource represents a source of node feature labels
type LabelSource interface {
	// Name returns a friendly name for this source
	Name() string

	// Discover returns discovered feature labels
	Discover() (FeatureLabels, error)

	// NewConfig returns a new default config of the source
	NewConfig() Config

	// GetConfig returns the effective configuration of the source
	GetConfig() Config

	// SetConfig changes the effective configuration of the source
	SetConfig(Config)
}

// Config is the generic interface for source configuration data
type Config interface {
}
