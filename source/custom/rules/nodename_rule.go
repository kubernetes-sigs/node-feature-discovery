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

	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/custom/expression"
	"sigs.k8s.io/node-feature-discovery/source/system"
)

// NodenameRule matches on nodenames configured in a ConfigMap
type NodenameRule struct {
	expression.MatchExpression
}

func (r *NodenameRule) Match() (bool, error) {
	nodeName, ok := source.GetFeatureSource("system").GetFeatures().Values[system.NameFeature].Elements["nodename"]
	if !ok || nodeName == "" {
		return false, fmt.Errorf("node name not available")
	}
	return r.MatchExpression.Match(true, nodeName)
}

func (r *NodenameRule) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.MatchExpression); err != nil {
		return err
	}
	// Force regexp matching
	if r.Op == expression.MatchIn {
		r.Op = expression.MatchInRegexp
	}
	// We need to run Validate() because operator forcing above
	return r.Validate()
}
