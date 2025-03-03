/*
Copyright 2025 The Kubernetes Authors.

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

package nodefeaturerule

import (
	"testing"

	"github.com/stretchr/testify/assert"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestGroupRule(t *testing.T) {
	f := &nfdv1alpha1.Features{}
	r1 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"var-1": "var-val-1"},
	}
	r2 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"var-2": "var-val-2"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.kf-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-1": newMatchExpression(nfdv1alpha1.MatchExists),
				},
			},
		},
	}

	// Test totally empty features
	m, err := ExecuteGroupRule(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Vars, m.Vars, "empty matcher should have matched empty features")

	m, err = ExecuteGroupRule(r2, f, true)
	assert.NoError(t, err, "matching against a missing feature should not have returned an error")
	assert.Empty(t, m.Vars)

	// Test properly initialized empty features
	f = nfdv1alpha1.NewFeatures()

	m, err = ExecuteGroupRule(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Vars, m.Vars, "empty matcher should have matched empty features")

	m, err = ExecuteGroupRule(r2, f, true)
	assert.NoError(t, err, "matching against a missing feature should not have returned an error")
	assert.Empty(t, m.Vars)

	// Test empty feature sets
	f.Flags["domain-1.kf-1"] = nfdv1alpha1.NewFlagFeatures()
	f.Attributes["domain-1.vf-1"] = nfdv1alpha1.NewAttributeFeatures(nil)
	f.Instances["domain-1.if-1"] = nfdv1alpha1.NewInstanceFeatures()

	m, err = ExecuteGroupRule(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Vars, m.Vars, "empty matcher should have matched empty features")

	m, err = ExecuteGroupRule(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Vars, "unexpected match")

	// Test non-empty feature sets
	f.Flags["domain-1.kf-1"].Elements["key-x"] = nfdv1alpha1.Nil{}
	f.Attributes["domain-1.vf-1"].Elements["key-1"] = "val-x"
	f.Instances["domain-1.if-1"] = nfdv1alpha1.NewInstanceFeatures(*nfdv1alpha1.NewInstanceFeature(map[string]string{"attr-1": "val-x"}))

	m, err = ExecuteGroupRule(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Vars, m.Vars, "empty matcher should have matched empty features")

	// Test empty MatchExpressions
	r1.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature:          "domain-1.kf-1",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{},
		},
	}
	m, err = ExecuteGroupRule(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Vars, m.Vars, "empty match expression set mathces anything")

	// Match "key" features
	m, err = ExecuteGroupRule(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Vars, "keys should not have matched")

	f.Flags["domain-1.kf-1"].Elements["key-1"] = nfdv1alpha1.Nil{}
	m, err = ExecuteGroupRule(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r2.Vars, m.Vars, "vars should be present")

	// Match "value" features
	r3 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"var-3": "var-val-3", "empty": ""},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.vf-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = ExecuteGroupRule(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Vars, "values should not have matched")

	f.Attributes["domain-1.vf-1"].Elements["key-1"] = "val-1"
	m, err = ExecuteGroupRule(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Vars, m.Vars, "values should have matched")

	// Match "instance" features
	r4 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"var-4": "var-val-4"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.if-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"attr-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = ExecuteGroupRule(r4, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Vars, "instances should not have matched")

	f.Instances["domain-1.if-1"].Elements[0].Attributes["attr-1"] = "val-1"
	m, err = ExecuteGroupRule(r4, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r4.Vars, m.Vars, "instances should have matched")

	// Match "multi-type" features
	f2 := nfdv1alpha1.NewFeatures()
	f2.Flags["dom.feat"] = nfdv1alpha1.NewFlagFeatures("k-1", "k-2")
	f2.Attributes["dom.feat"] = nfdv1alpha1.NewAttributeFeatures(map[string]string{"a-1": "v-1", "a-2": "v-2"})
	f2.Instances["dom.feat"] = nfdv1alpha1.NewInstanceFeatures(
		*nfdv1alpha1.NewInstanceFeature(map[string]string{"ia-1": "iv-1"}),
		*nfdv1alpha1.NewInstanceFeature(map[string]string{"ia-2": "iv-2"}),
	)

	r5 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"feat": "val-1"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "dom.feat",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"k-1": newMatchExpression(nfdv1alpha1.MatchExists),
				},
			},
		},
	}
	m, err = ExecuteGroupRule(r5, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Vars, m.Vars, "key in multi-type feature should have matched")

	r5.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"a-1": newMatchExpression(nfdv1alpha1.MatchIn, "v-1"),
			},
		},
	}
	m, err = ExecuteGroupRule(r5, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Vars, m.Vars, "attribute in multi-type feature should have matched")

	r5.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"ia-1": newMatchExpression(nfdv1alpha1.MatchIn, "iv-1"),
			},
		},
	}
	m, err = ExecuteGroupRule(r5, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Vars, m.Vars, "attribute in multi-type feature should have matched")

	r5.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"k-2": newMatchExpression(nfdv1alpha1.MatchExists),
				"a-2": newMatchExpression(nfdv1alpha1.MatchIn, "v-2"),
			},
		},
	}
	m, err = ExecuteGroupRule(r5, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Vars, m.Vars, "features in multi-type feature should have matched flags and attributes")

	r5.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"ia-2": newMatchExpression(nfdv1alpha1.MatchIn, "iv-2"),
			},
		},
	}
	m, err = ExecuteGroupRule(r5, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Vars, m.Vars, "features in multi-type feature should have matched instance")

	// Test multiple feature matchers
	r6 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"var-6": "var-val-6"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.vf-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-x"),
				},
			},
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.if-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"attr-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = ExecuteGroupRule(r6, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Vars, "instances should not have matched")

	(*r6.MatchFeatures[0].MatchExpressions)["key-1"] = newMatchExpression(nfdv1alpha1.MatchIn, "val-1")
	m, err = ExecuteGroupRule(r6, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r6.Vars, m.Vars, "instances should have matched")

	// Test MatchAny
	r6.MatchAny = []nfdv1alpha1.MatchAnyElem{
		{
			MatchFeatures: nfdv1alpha1.FeatureMatcher{
				nfdv1alpha1.FeatureMatcherTerm{
					Feature: "domain-1.kf-1",
					MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
						"key-na": newMatchExpression(nfdv1alpha1.MatchExists),
					},
				},
			},
		},
	}
	m, err = ExecuteGroupRule(r6, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Vars, "instances should not have matched")

	r6.MatchAny = append(r6.MatchAny,
		nfdv1alpha1.MatchAnyElem{
			MatchFeatures: nfdv1alpha1.FeatureMatcher{
				nfdv1alpha1.FeatureMatcherTerm{
					Feature: "domain-1.kf-1",
					MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
						"key-1": newMatchExpression(nfdv1alpha1.MatchExists),
					},
				},
			},
		})
	(*r3.MatchFeatures[0].MatchExpressions)["key-1"] = newMatchExpression(nfdv1alpha1.MatchIn, "val-1")
	m, err = ExecuteGroupRule(r6, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r6.Vars, m.Vars, "instances should have matched")
}

func TestGroupRuleTemplating(t *testing.T) {
	f := &nfdv1alpha1.Features{
		Flags: map[string]nfdv1alpha1.FlagFeatureSet{
			"domain_1.kf_1": {
				Elements: map[string]nfdv1alpha1.Nil{
					"key-a": {},
					"key-b": {},
					"key-c": {},
				},
			},
			"domain.mf": {
				Elements: map[string]nfdv1alpha1.Nil{
					"key-a": {},
					"key-b": {},
					"key-c": {},
				},
			},
		},
		Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{
			"domain_1.vf_1": {
				Elements: map[string]string{
					"key-1": "val-1",
					"keu-2": "val-2",
					"key-3": "val-3",
					"key-4": "val-4",
				},
			},
			"domain.mf": {
				Elements: map[string]string{
					"key-d": "val-d",
					"key-e": "val-e",
				},
			},
		},
		Instances: map[string]nfdv1alpha1.InstanceFeatureSet{
			"domain_1.if_1": {
				Elements: []nfdv1alpha1.InstanceFeature{
					{
						Attributes: map[string]string{
							"attr-1": "1",
							"attr-2": "val-2",
						},
					},
					{
						Attributes: map[string]string{
							"attr-1": "10",
							"attr-2": "val-20",
						},
					},
					{
						Attributes: map[string]string{
							"attr-1": "100",
							"attr-2": "val-200",
						},
					},
					{
						Attributes: map[string]string{
							"attr-1": "1000",
							"attr-2": "val-2000",
							"attr-3": "3000",
						},
					},
				},
			},
		},
	}

	r1 := &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"var-1": "var-val-1"},
		VarsTemplate: `
var-1=will-be-overridden
var-2=
{{range .domain_1.kf_1}}kf-{{.Name}}=true
{{end}}
{{range .domain_1.vf_1}}vf-{{.Name}}=vf-{{.Value}}
{{end}}
{{range .domain_1.if_1}}if-{{index . "attr-1"}}_{{index . "attr-2"}}=present
{{end}}`,
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain_1.kf_1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-a": newMatchExpression(nfdv1alpha1.MatchExists),
					"key-c": newMatchExpression(nfdv1alpha1.MatchExists),
					"foo":   newMatchExpression(nfdv1alpha1.MatchDoesNotExist),
				},
			},
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain_1.vf_1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-1", "val-2"),
					"bar":   newMatchExpression(nfdv1alpha1.MatchDoesNotExist),
				},
			},
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain_1.if_1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"attr-1": newMatchExpression(nfdv1alpha1.MatchLt, "100"),
				},
			},
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain_1.if_1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"attr-1": newMatchExpression(nfdv1alpha1.MatchExists),
					"attr-2": newMatchExpression(nfdv1alpha1.MatchExists),
					"attr-3": newMatchExpression(nfdv1alpha1.MatchExists),
				},
			},
		},
	}

	// MatchAny, empty MatchFeatures
	r2 := r1.DeepCopy()
	r2.MatchAny = []nfdv1alpha1.MatchAnyElem{{MatchFeatures: r2.MatchFeatures}}
	r2.MatchFeatures = nil

	expectedVars := map[string]string{
		"var-1": "var-val-1",
		"var-2": "",
		// From kf_1 template
		"kf-key-a": "true",
		"kf-key-c": "true",
		"kf-foo":   "true",
		// From vf_1 template
		"vf-key-1": "vf-val-1",
		"vf-bar":   "vf-",
		// From if_1 template
		"if-1_val-2":       "present",
		"if-10_val-20":     "present",
		"if-1000_val-2000": "present",
	}

	m, err := ExecuteGroupRule(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	m, err = ExecuteGroupRule(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	// Test "multi-type" feature
	r3 := &nfdv1alpha1.GroupRule{
		VarsTemplate: `
{{range .domain.mf}}mf-{{.Name}}=found
{{end}}`,
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain.mf",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-a": newMatchExpression(nfdv1alpha1.MatchExists),
					"key-d": newMatchExpression(nfdv1alpha1.MatchIn, "val-d"),
				},
			},
		},
	}

	expectedVars = map[string]string{
		"mf-key-a": "found",
		"mf-key-d": "found",
	}
	m, err = ExecuteGroupRule(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	//
	// Test error cases
	//
	r4 := &nfdv1alpha1.GroupRule{
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			// We need at least one matcher to match to execute the template.
			// Use a simple empty matchexpression set to match anything.
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain_1.kf_1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-a": newMatchExpression(nfdv1alpha1.MatchExists),
				},
			},
		},
	}

	r4.VarsTemplate = "foo=bar"
	m, err = ExecuteGroupRule(r4, f, true)
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, m.Vars, "instances should have matched")

	r4.VarsTemplate = "foo"
	_, err = ExecuteGroupRule(r4, f, true)
	assert.Error(t, err)

	r4.VarsTemplate = "{{"
	_, err = ExecuteGroupRule(r4, f, true)
	assert.Error(t, err)

	//
	// Test matchName
	//
	r5 := &nfdv1alpha1.GroupRule{
		VarsTemplate: "{{range .domain_1.vf_1}}{{.Name}}={{.Value}}\n{{end}}",
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain_1.vf_1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-5": newMatchExpression(nfdv1alpha1.MatchDoesNotExist),
				},
				MatchName: newMatchExpression(nfdv1alpha1.MatchIn, "key-1", "key-4"),
			},
		},
	}
	expectedVars = map[string]string{
		"key-1": "val-1",
		"key-4": "val-4",
		"key-5": "",
	}

	m, err = ExecuteGroupRule(r5, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	r5 = &nfdv1alpha1.GroupRule{
		Vars: map[string]string{"should-not-match": "true"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature:   "domain_1.vf_1",
				MatchName: newMatchExpression(nfdv1alpha1.MatchIn, "key-not-exists"),
			},
		},
	}

	m, err = ExecuteGroupRule(r5, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, map[string]string(nil), m.Vars, "instances should have matched")
}
