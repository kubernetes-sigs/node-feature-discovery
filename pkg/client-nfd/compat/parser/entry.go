/*
Copyright 2024 The Kubernetes Authors.

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

package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	semver "github.com/Masterminds/semver/v3"

	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
)

type ValueType string

func (t ValueType) String() string {
	return string(t)
}

const (
	LiteralValueType ValueType = "literal"
	RangeValueType   ValueType = "range"
)

type EntryGetter interface {
	KeyRaw() string
	Source() string
	Feature() string
	Option() string
	ValueRaw() interface{}
	ValueType() ValueType
	ValueEqualTo(source.FeatureLabelValue) (bool, error)
}

type CompatibilityEntry struct {
	// Key
	keyRaw  string
	source  string
	feature string
	option  string

	// Value
	value CompatibilityValue
}

type CompatibilityValue struct {
	raw       interface{}
	kind      ValueType
	intervals []string
}

func (c *CompatibilityEntry) KeyRaw() string {
	return c.keyRaw
}

func (c *CompatibilityEntry) Source() string {
	return c.source
}

func (c *CompatibilityEntry) Feature() string {
	return c.feature
}

func (c *CompatibilityEntry) Option() string {
	return c.option
}

func (c *CompatibilityEntry) ValueRaw() interface{} {
	return c.value.raw
}

func (c *CompatibilityEntry) ValueType() ValueType {
	return c.value.kind
}

func (c *CompatibilityEntry) ValueEqualTo(val source.FeatureLabelValue) (bool, error) {
	switch c.value.kind {
	case LiteralValueType:
		return c.equalTo(val)
	case RangeValueType:
		return c.inRange(val)
	default:
		return false, fmt.Errorf("unknown type: %v", c.value.kind)
	}
}

func (c *CompatibilityEntry) equalTo(val interface{}) (bool, error) {
	switch v := val.(type) {
	case string:
		// NFD label boolean values are converted to string.
		// It may be required to keep it this way for k8s labelling.
		// TODO: either fix this in NFD or leave it as it is.
		valB, err := strconv.ParseBool(v)
		if err == nil {
			cvB, ok := c.value.raw.(bool)
			if ok {
				return cvB == valB, nil
			}
			return false, nil
		}
		return c.value.raw == val, nil
	case bool:
		return c.value.raw == val, nil
	default:
		return false, fmt.Errorf("only string and boolean types are allowed for comparison")
	}
}

func (c *CompatibilityEntry) inRange(iface interface{}) (bool, error) {
	val, ok := iface.(string)
	if !ok {
		return false, fmt.Errorf("only string type is allowed to compare value to ranges")
	}
	if c.Source() == kernel.Name && c.Feature() == kernel.VersionFeature {
		return c.inRangeKernelVersion(val)
	}
	return c.inRangeSemanticVersion(val)
}

func (c *CompatibilityEntry) inRangeKernelVersion(val string) (bool, error) {
	kernelVersion, kernelFlavor := parseKernelVersion(val)
	v, err := semver.NewVersion(kernelVersion)
	if err != nil {
		return false, err
	}

	for _, interval := range c.value.intervals {
		s := strings.SplitN(interval, ",", 2)
		start, sFlavor := parseKernelVersion(s[0])
		constraint := start

		if len(s) > 1 {
			end, eFlavor := parseKernelVersion(s[1])
			constraint += fmt.Sprintf(",%s", end)
			if sFlavor != eFlavor {
				return false, fmt.Errorf("flavor of the kernel must be the same in the specified interval, currnently have %q", interval)
			}
		}

		flavorMatched, err := regexp.MatchString(sFlavor, kernelFlavor)
		if err != nil {
			return false, err
		} else if !flavorMatched {
			continue
		}

		nc, err := semver.NewConstraint(constraint)
		if err != nil {
			return false, err
		}
		valid, _ := nc.Validate(v)
		if valid {
			return true, nil
		}
	}

	return false, nil
}

func (c *CompatibilityEntry) inRangeSemanticVersion(val string) (bool, error) {
	v, err := semver.NewVersion(val)
	if err != nil {
		return false, err
	}

	for _, constraint := range c.value.intervals {
		nc, err := semver.NewConstraint(constraint)
		if err != nil {
			return false, err
		}

		valid, _ := nc.Validate(v)
		if valid {
			return true, nil
		}
	}

	return false, nil
}

func parseKernelVersion(raw string) (version string, flavor string) {
	s := strings.SplitN(raw, "-", 2)
	version = s[0]

	if len(s) > 1 {
		flavor = s[1]
	}

	return
}
