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

// Value of a feature
type FeatureValue interface {
}

// Boolean feature value
type BoolFeatureValue bool

func (b BoolFeatureValue) String() string {
	if b {
		return "true"
	}
	return "false"
}

type Features map[string]FeatureValue

// FeatureSource represents a source of a discovered node feature.
type FeatureSource interface {
	// Name returns a friendly name for this source of node feature.
	Name() string

	// Discover returns discovered features for this node.
	Discover() (Features, error)

	// NewConfig returns a new default config of the source
	NewConfig() Config

	// GetConfig returns the effective configuration of the source
	GetConfig() Config

	// SetConfig changes the effective configuration of the source
	SetConfig(Config)
}

type Config interface {
}
