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

package rules

import (
	"encoding/json"
	"fmt"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/system"
)

// NodenameRule matches on nodenames configured in a ConfigMap
type NodenameRule struct {
	nfdv1alpha1.MatchExpression
}

// Match checks if node name matches the rule.
func (r *NodenameRule) Match() (bool, error) {
	nodeName, ok := source.GetFeatureSource("system").GetFeatures().Attributes[system.NameFeature].Elements["nodename"]
	if !ok || nodeName == "" {
		return false, fmt.Errorf("node name not available")
	}
	return r.MatchExpression.Match(true, nodeName)
}

// UnmarshalJSON unmarshals and validates the data provided
func (r *NodenameRule) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.MatchExpression); err != nil {
		return err
	}
	// Force regexp matching
	if r.Op == nfdv1alpha1.MatchIn {
		r.Op = nfdv1alpha1.MatchInRegexp
	}
	// We need to run Validate() because operator forcing above
	return r.Validate()
}
