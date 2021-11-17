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

import (
	"strings"

	"fmt"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
)

// Execute the rule against a set of input features.
func (r *Rule) Execute(features map[string]*feature.DomainFeatures) (map[string]string, error) {
	if len(r.MatchAny) > 0 {
		// Logical OR over the matchAny matchers
		matched := false
		for _, matcher := range r.MatchAny {
			if m, err := matcher.match(features); err != nil {
				return nil, err
			} else if m {
				matched = true
				break
			}
		}
		if !matched {
			return nil, nil
		}
	}

	if len(r.MatchFeatures) > 0 {
		if m, err := r.MatchFeatures.match(features); err != nil {
			return nil, err
		} else if !m {
			return nil, nil
		}
	}

	labels := make(map[string]string, len(r.Labels))
	for k, v := range r.Labels {
		labels[k] = v
	}

	return labels, nil
}

func (e *MatchAnyElem) match(features map[string]*feature.DomainFeatures) (bool, error) {
	return e.MatchFeatures.match(features)
}

func (m *FeatureMatcher) match(features map[string]*feature.DomainFeatures) (bool, error) {
	// Logical AND over the terms
	for _, term := range *m {
		split := strings.SplitN(term.Feature, ".", 2)
		if len(split) != 2 {
			return false, fmt.Errorf("invalid selector %q: must be <domain>.<feature>", term.Feature)
		}
		domain := split[0]
		// Ignore case
		featureName := strings.ToLower(split[1])

		domainFeatures, ok := features[domain]
		if !ok {
			return false, fmt.Errorf("unknown feature source/domain %q", domain)
		}

		var m bool
		var err error
		if f, ok := domainFeatures.Keys[featureName]; ok {
			m, err = term.MatchExpressions.MatchKeys(f.Elements)
		} else if f, ok := domainFeatures.Values[featureName]; ok {
			m, err = term.MatchExpressions.MatchValues(f.Elements)
		} else if f, ok := domainFeatures.Instances[featureName]; ok {
			m, err = term.MatchExpressions.MatchInstances(f.Elements)
		} else {
			return false, fmt.Errorf("%q feature of source/domain %q not available", featureName, domain)
		}

		if err != nil {
			return false, err
		} else if !m {
			return false, nil
		}
	}
	return true, nil
}
