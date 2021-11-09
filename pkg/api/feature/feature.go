/*
Copyright 2021 The Kubernetes Authors.

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

package feature

// NewDomainFeatures creates a new instance of Features, initializing specified
// features to empty values
func NewDomainFeatures() *DomainFeatures {
	return &DomainFeatures{
		Keys:      make(map[string]*KeyFeatureSet),
		Values:    make(map[string]*ValueFeatureSet),
		Instances: make(map[string]*InstanceFeatureSet)}
}

func NewKeyFeatures() *KeyFeatureSet { return &KeyFeatureSet{Elements: make(map[string]Nil)} }

func NewValueFeatures() *ValueFeatureSet { return &ValueFeatureSet{Elements: make(map[string]string)} }

func NewInstanceFeatures() *InstanceFeatureSet { return &InstanceFeatureSet{} }

func NewInstanceFeature() *InstanceFeature {
	return &InstanceFeature{Attributes: make(map[string]string)}
}
