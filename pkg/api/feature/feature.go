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
		Keys:      make(map[string]KeyFeatureSet),
		Values:    make(map[string]ValueFeatureSet),
		Instances: make(map[string]InstanceFeatureSet)}
}

// NewKeyFeatures creates a new instance of KeyFeatureSet.
func NewKeyFeatures(keys ...string) KeyFeatureSet {
	e := make(map[string]Nil, len(keys))
	for _, k := range keys {
		e[k] = Nil{}
	}
	return KeyFeatureSet{Elements: e}
}

// NewValueFeatures creates a new instance of ValueFeatureSet.
func NewValueFeatures(values map[string]string) ValueFeatureSet {
	if values == nil {
		values = make(map[string]string)
	}
	return ValueFeatureSet{Elements: values}
}

// NewInstanceFeatures creates a new instance of InstanceFeatureSet.
func NewInstanceFeatures(instances []InstanceFeature) InstanceFeatureSet {
	return InstanceFeatureSet{Elements: instances}
}

// NewInstanceFeature creates a new InstanceFeature instance.
func NewInstanceFeature(attrs map[string]string) *InstanceFeature {
	if attrs == nil {
		attrs = make(map[string]string)
	}
	return &InstanceFeature{Attributes: attrs}
}

// InsertFeatureValues inserts new values into a specific feature.
func InsertFeatureValues(f Features, domain, feature string, values map[string]string) {
	if _, ok := f[domain]; !ok {
		f[domain] = NewDomainFeatures()
	}
	if _, ok := f[domain].Values[feature]; !ok {
		f[domain].Values[feature] = NewValueFeatures(values)
		return
	}

	for k, v := range values {
		f[domain].Values[feature].Elements[k] = v
	}
}
