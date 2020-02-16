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
	"encoding/json"

	"sigs.k8s.io/node-feature-discovery/source"
)

type MatchRule struct {
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
	return features, nil
}
