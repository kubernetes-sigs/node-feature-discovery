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

package v1alpha1

// NewFeatures creates a new instance of Features, initializing all feature
// types (flags, attributes and instances) to empty values.
func NewFeatures() *Features {
	return &Features{
		Flags:      make(map[string]FlagFeatureSet),
		Attributes: make(map[string]AttributeFeatureSet),
		Instances:  make(map[string]InstanceFeatureSet)}
}

// NewFlagFeatures creates a new instance of KeyFeatureSet.
func NewFlagFeatures(keys ...string) FlagFeatureSet {
	e := make(map[string]Nil, len(keys))
	for _, k := range keys {
		e[k] = Nil{}
	}
	return FlagFeatureSet{Elements: e}
}

// NewAttributeFeatures creates a new instance of ValueFeatureSet.
func NewAttributeFeatures(values map[string]string) AttributeFeatureSet {
	if values == nil {
		values = make(map[string]string)
	}
	return AttributeFeatureSet{Elements: values}
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

// InsertAttributeFeatures inserts new values into a specific feature.
func (f *Features) InsertAttributeFeatures(domain, feature string, values map[string]string) {
	key := domain + "." + feature
	if _, ok := f.Attributes[key]; !ok {
		f.Attributes[key] = NewAttributeFeatures(values)
		return
	}

	for k, v := range values {
		f.Attributes[key].Elements[k] = v
	}
}

// Exists returns a non-empty string if a feature exists. The return value is
// the type of the feautre, i.e. "flag", "attribute" or "instance".
func (f *Features) Exists(name string) string {
	if _, ok := f.Flags[name]; ok {
		return "flag"
	}
	if _, ok := f.Attributes[name]; ok {
		return "attribute"
	}
	if _, ok := f.Instances[name]; ok {
		return "instance"
	}
	return ""
}
