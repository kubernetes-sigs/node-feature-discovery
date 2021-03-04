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

package custom

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/source/custom/expression"
)

func TestRule(t *testing.T) {
	f := map[string]*feature.DomainFeatures{}
	r1 := Rule{Labels: map[string]string{"label-1": "", "label-2": "true"}}
	r2 := Rule{
		Labels: map[string]string{"label-1": "label-val-1"},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature:          "domain-1.kf-1",
				MatchExpressions: expression.MatchExpressionSet{"key-1": expression.MustCreateMatchExpression(expression.MatchExists)},
			},
		},
	}

	// Test totally empty features
	m, err := r1.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m, "empty matcher should have matched empty features")

	_, err = r2.execute(f)
	assert.Error(t, err, "matching agains a missing domain should have returned an error")

	// Test empty domain
	d := feature.NewDomainFeatures()
	f["domain-1"] = d

	m, err = r1.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m, "empty matcher should have matched empty features")

	_, err = r2.execute(f)
	assert.Error(t, err, "matching agains a missing feature type should have returned an error")

	// Test empty feature sets
	d.Keys["kf-1"] = feature.NewKeyFeatures()
	d.Values["vf-1"] = feature.NewValueFeatures(nil)
	d.Instances["if-1"] = feature.NewInstanceFeatures(nil)

	m, err = r1.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m, "empty matcher should have matched empty features")

	m, err = r2.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m, "unexpected match")

	// Test non-empty feature sets
	d.Keys["kf-1"].Elements["key-x"] = feature.Nil{}
	d.Values["vf-1"].Elements["key-1"] = "val-x"
	d.Instances["if-1"] = feature.NewInstanceFeatures([]feature.InstanceFeature{
		*feature.NewInstanceFeature(map[string]string{"attr-1": "val-x"})})

	m, err = r1.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m, "empty matcher should have matched empty features")

	// Match "key" features
	m, err = r2.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m, "keys should not have matched")

	d.Keys["kf-1"].Elements["key-1"] = feature.Nil{}
	m, err = r2.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r2.Labels, m, "keys should have matched")

	// Match "value" features
	r3 := Rule{
		Labels: map[string]string{"label-3": "label-val-3", "empty": ""},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature:          "domain-1.vf-1",
				MatchExpressions: expression.MatchExpressionSet{"key-1": expression.MustCreateMatchExpression(expression.MatchIn, "val-1")},
			},
		},
	}
	m, err = r3.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m, "values should not have matched")

	d.Values["vf-1"].Elements["key-1"] = "val-1"
	m, err = r3.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m, "values should have matched")

	// Match "instance" features
	r4 := Rule{
		Labels: map[string]string{"label-4": "label-val-4"},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature:          "domain-1.if-1",
				MatchExpressions: expression.MatchExpressionSet{"attr-1": expression.MustCreateMatchExpression(expression.MatchIn, "val-1")},
			},
		},
	}
	m, err = r4.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m, "instances should not have matched")

	d.Instances["if-1"].Elements[0].Attributes["attr-1"] = "val-1"
	m, err = r4.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r4.Labels, m, "instances should have matched")

	// Test multiple feature matchers
	r5 := Rule{
		Labels: map[string]string{"label-5": "label-val-5"},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature:          "domain-1.vf-1",
				MatchExpressions: expression.MatchExpressionSet{"key-1": expression.MustCreateMatchExpression(expression.MatchIn, "val-x")},
			},
			FeatureMatcherTerm{
				Feature:          "domain-1.if-1",
				MatchExpressions: expression.MatchExpressionSet{"attr-1": expression.MustCreateMatchExpression(expression.MatchIn, "val-1")},
			},
		},
	}
	m, err = r5.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m, "instances should not have matched")

	r5.MatchFeatures[0].MatchExpressions["key-1"] = expression.MustCreateMatchExpression(expression.MatchIn, "val-1")
	m, err = r5.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Labels, m, "instances should have matched")

	// Test MatchAny
	r5.MatchAny = []MatchAnyElem{
		MatchAnyElem{
			MatchFeatures: FeatureMatcher{
				FeatureMatcherTerm{
					Feature:          "domain-1.kf-1",
					MatchExpressions: expression.MatchExpressionSet{"key-na": expression.MustCreateMatchExpression(expression.MatchExists)},
				},
			},
		},
	}
	m, err = r5.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m, "instances should not have matched")

	r5.MatchAny = append(r5.MatchAny,
		MatchAnyElem{
			MatchFeatures: FeatureMatcher{
				FeatureMatcherTerm{
					Feature:          "domain-1.kf-1",
					MatchExpressions: expression.MatchExpressionSet{"key-1": expression.MustCreateMatchExpression(expression.MatchExists)},
				},
			},
		})
	r5.MatchFeatures[0].MatchExpressions["key-1"] = expression.MustCreateMatchExpression(expression.MatchIn, "val-1")
	m, err = r5.execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Labels, m, "instances should have matched")
}
