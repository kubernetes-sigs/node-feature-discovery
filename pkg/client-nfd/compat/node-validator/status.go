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

package nodevalidator

import (
	compatv1alpha1 "sigs.k8s.io/node-feature-discovery/api/image-compatibility/v1alpha1"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

// CompatibilityStatus represents the state of
// feature matching between the image and the host.
type CompatibilityStatus struct {
	// Rules contain information about the matching status
	// of all Node Feature Rules.
	Rules []RuleStatus `json:"rules"`
	// Description of the compatibility set.
	Description string `json:"description,omitempty"`
	// Weight provides information about the priority of the compatibility set.
	Weight int `json:"weight,omitempty"`
	// Tag provides information about the tag assigned to the compatibility set.
	Tag string `json:"tag,omitempty"`
}

func newCompatibilityStatus(c *compatv1alpha1.Compatibility) CompatibilityStatus {
	cs := CompatibilityStatus{
		Description: c.Description,
		Weight:      c.Weight,
		Tag:         c.Tag,
	}

	return cs
}

// RuleStatus contains information about features matching.
type RuleStatus struct {
	*nfdv1alpha1.Rule
	// IsMatch provides information if the rule matches with the host.
	IsMatch bool `json:"isMatch"`
}
