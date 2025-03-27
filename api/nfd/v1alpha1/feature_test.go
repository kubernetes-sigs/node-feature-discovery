/*
Copyright 2023 The Kubernetes Authors.

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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagFeatureSet(t *testing.T) {
	f1 := FlagFeatureSet{}
	f2 := FlagFeatureSet{}
	var expectedElems map[string]Nil = nil

	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewFlagFeatures()
	expectedElems = make(map[string]Nil)
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewFlagFeatures("k1")
	expectedElems["k1"] = Nil{}
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewFlagFeatures("k2")
	expectedElems["k2"] = Nil{}
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)
}

func TestAttributeFeatureSet(t *testing.T) {
	f1 := AttributeFeatureSet{}
	f2 := AttributeFeatureSet{}
	var expectedElems map[string]string = nil

	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewAttributeFeatures(map[string]string{})
	expectedElems = make(map[string]string)
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewAttributeFeatures(map[string]string{"k1": "v1"})
	expectedElems["k1"] = "v1"
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewAttributeFeatures(map[string]string{"k2": "v2"})
	expectedElems["k2"] = "v2"
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewAttributeFeatures(map[string]string{"k1": "v1.overridden", "k3": "v3"})
	expectedElems["k1"] = "v1.overridden"
	expectedElems["k3"] = "v3"
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)
}

func TestInstanceFeatureSet(t *testing.T) {
	f1 := InstanceFeatureSet{}
	f2 := InstanceFeatureSet{}
	var expectedElems []InstanceFeature = nil

	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewInstanceFeatures()
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewInstanceFeatures(InstanceFeature{})
	expectedElems = append(expectedElems, InstanceFeature{})
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2 = NewInstanceFeatures(InstanceFeature{
		Attributes: map[string]string{
			"a1": "v1",
			"a2": "v2",
		},
	})
	expectedElems = append(expectedElems, *NewInstanceFeature(map[string]string{"a1": "v1", "a2": "v2"}))
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)

	f2.Elements[0].Attributes["a2"] = "v2.2"
	expectedElems = append(expectedElems, *NewInstanceFeature(map[string]string{"a1": "v1", "a2": "v2.2"}))
	f2.MergeInto(&f1)
	assert.Equal(t, expectedElems, f1.Elements)
}

func TestFeature(t *testing.T) {
	f := Features{}

	// Test InsertAttributeFeatures()
	f.InsertAttributeFeatures("dom", "attr", map[string]string{"k1": "v1", "k2": "v2"})
	expectedAttributes := map[string]string{"k1": "v1", "k2": "v2"}
	assert.Equal(t, expectedAttributes, f.Attributes["dom.attr"].Elements)

	f.InsertAttributeFeatures("dom", "attr", map[string]string{"k2": "v2.override", "k3": "v3"})
	expectedAttributes["k2"] = "v2.override"
	expectedAttributes["k3"] = "v3"
	assert.Equal(t, expectedAttributes, f.Attributes["dom.attr"].Elements)

	// Test merging
	f = Features{}
	f2 := Features{}
	expectedFeatures := Features{}

	f2.MergeInto(&f)
	assert.Equal(t, expectedFeatures, f)

	f2 = *NewFeatures()
	f2.Flags["dom.flag"] = NewFlagFeatures("k1", "k2")
	f2.Attributes["dom.attr"] = NewAttributeFeatures(map[string]string{"k1": "v1", "k2": "v2"})
	f2.Instances["dom.inst"] = NewInstanceFeatures(
		*NewInstanceFeature(map[string]string{"a1": "v1.1", "a2": "v1.2"}),
		*NewInstanceFeature(map[string]string{"a1": "v2.1", "a2": "v2.2"}),
	)
	f2.MergeInto(&f)
	assert.Equal(t, f2, f)

	f2.Flags["dom.flag"] = NewFlagFeatures("k3")
	f2.Attributes["dom.attr"] = NewAttributeFeatures(map[string]string{"k1": "v1.override"})
	f2.Instances["dom.inst"] = NewInstanceFeatures(*NewInstanceFeature(map[string]string{"a1": "v3.1", "a3": "v3.3"}))
	f2.MergeInto(&f)
	expectedFeatures = *NewFeatures()
	expectedFeatures.Flags["dom.flag"] = FlagFeatureSet{Elements: map[string]Nil{"k1": {}, "k2": {}, "k3": {}}}
	expectedFeatures.Attributes["dom.attr"] = AttributeFeatureSet{Elements: map[string]string{"k1": "v1.override", "k2": "v2"}}
	expectedFeatures.Instances["dom.inst"] = InstanceFeatureSet{
		Elements: []InstanceFeature{
			{Attributes: map[string]string{"a1": "v1.1", "a2": "v1.2"}},
			{Attributes: map[string]string{"a1": "v2.1", "a2": "v2.2"}},
			{Attributes: map[string]string{"a1": "v3.1", "a3": "v3.3"}},
		},
	}
	assert.Equal(t, expectedFeatures, f)
}

func TestFeatureSpec(t *testing.T) {
	// Test merging
	f := NodeFeatureSpec{}
	f2 := NodeFeatureSpec{}
	expectedFeatures := NodeFeatureSpec{}

	f2.MergeInto(&f)
	assert.Equal(t, expectedFeatures, f)

	f2 = *NewNodeFeatureSpec()
	f2.Labels = map[string]string{"l1": "v1", "l2": "v2"}
	f2.Features = *NewFeatures()
	f2.Features.Flags["dom.flag"] = NewFlagFeatures("k1", "k2")

	expectedFeatures = *f2.DeepCopy()
	f2.MergeInto(&f)
	assert.Equal(t, expectedFeatures, f)

	// Check that second merge updates the object correctly
	f2 = *NewNodeFeatureSpec()
	f2.Labels = map[string]string{"l1": "v1.override", "l3": "v3"}
	f2.Features = *NewFeatures()
	f2.Features.Flags["dom.flag2"] = NewFlagFeatures("k3")

	expectedFeatures.Labels["l1"] = "v1.override"
	expectedFeatures.Labels["l3"] = "v3"
	expectedFeatures.Features.Flags["dom.flag2"] = FlagFeatureSet{Elements: map[string]Nil{"k3": {}}}

	f2.MergeInto(&f)
	assert.Equal(t, expectedFeatures, f)
}
