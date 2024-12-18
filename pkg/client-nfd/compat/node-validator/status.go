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

// MatcherType represents a type of the used matcher.
type MatcherType string

const (
	// MatchExpressionType represents a matchExpression type.
	MatchExpressionType MatcherType = "matchExpression"
	// MatchNameType represents a matchName type.
	MatchNameType MatcherType = "matchName"
)

// CompatibilityStatus represents the state of
// feature matching between the image and the host.
type CompatibilityStatus struct {
	// Rules contain information about the matching status
	// of all Node Feature Rules.
	Rules []ProcessedRuleStatus `json:"rules"`
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

// ProcessedRuleStatus provides information whether the expressions succeeded on the host.
type ProcessedRuleStatus struct {
	// Name of the rule.
	Name string `json:"name"`
	// IsMatch provides information if the rule matches with the host.
	IsMatch bool `json:"isMatch"`

	// MatchedExpressions represents the expressions that succeed on the host.
	MatchedExpressions []MatchedExpression `json:"matchedExpressions,omitempty"`
	// MatchAny represents an array of logical OR conditions between MatchedExpressions entries.
	MatchedAny []MatchAnyElem `json:"matchedAny,omitempty"`
}

// MatchAnyElem represents a single object of MatchAny that contains MatchedExpression entries.
type MatchAnyElem struct {
	// MatchedExpressions contains MatchedExpression entries.
	MatchedExpressions []MatchedExpression `json:"matchedExpressions"`
}

// MatchedExpression represent all details about the expression that succeeded on the host.
type MatchedExpression struct {
	// Feature which is available to be evaluated on the host.
	Feature string `json:"feature"`
	// Name of the element.
	Name string `json:"name"`
	// Expression represents the expression provided by users.
	Expression *nfdv1alpha1.MatchExpression `json:"expression"`
	// MatcherType represents the matcher type, e.g. MatchExpression, MatchName.
	MatcherType MatcherType `json:"matcherType"`
	// IsMatch provides information whether the expression suceeded on the host.
	IsMatch bool `json:"isMatch"`
}
