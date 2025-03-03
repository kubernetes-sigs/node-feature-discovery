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

package nodefeaturerule

import (
	"testing"

	"github.com/stretchr/testify/assert"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

// newMatchExpression returns a new MatchExpression instance.
func newMatchExpression(op nfdv1alpha1.MatchOp, values ...string) *nfdv1alpha1.MatchExpression {
	return &nfdv1alpha1.MatchExpression{
		Op:    op,
		Value: values,
	}
}

func TestRule(t *testing.T) {
	f := &nfdv1alpha1.Features{}
	r1 := &nfdv1alpha1.Rule{Labels: map[string]string{"label-1": "", "label-2": "true"}}
	r2 := &nfdv1alpha1.Rule{
		Labels: map[string]string{"label-1": "label-val-1"},
		Vars:   map[string]string{"var-1": "var-val-1"},
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
	m, err := Execute(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")

	m, err = Execute(r2, f, true)
	assert.NoError(t, err, "matching against a missing feature should not have returned an error")
	assert.Empty(t, m.Labels)
	assert.Empty(t, m.Vars)

	// Test properly initialized empty features
	f = nfdv1alpha1.NewFeatures()

	m, err = Execute(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")
	assert.Empty(t, m.Vars, "vars should be empty")

	m, err = Execute(r2, f, true)
	assert.NoError(t, err, "matching against a missing feature should not have returned an error")
	assert.Empty(t, m.Labels)
	assert.Empty(t, m.Vars)

	// Test empty feature sets
	f.Flags["domain-1.kf-1"] = nfdv1alpha1.NewFlagFeatures()
	f.Attributes["domain-1.vf-1"] = nfdv1alpha1.NewAttributeFeatures(nil)
	f.Instances["domain-1.if-1"] = nfdv1alpha1.NewInstanceFeatures()

	m, err = Execute(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")

	m, err = Execute(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "unexpected match")

	// Test non-empty feature sets
	f.Flags["domain-1.kf-1"].Elements["key-x"] = nfdv1alpha1.Nil{}
	f.Attributes["domain-1.vf-1"].Elements["key-1"] = "val-x"
	f.Instances["domain-1.if-1"] = nfdv1alpha1.NewInstanceFeatures(*nfdv1alpha1.NewInstanceFeature(map[string]string{"attr-1": "val-x"}))

	m, err = Execute(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")

	// Test empty MatchExpressions
	r1.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature:          "domain-1.kf-1",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{},
		},
	}
	m, err = Execute(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty match expression set mathces anything")

	// Match "key" features
	m, err = Execute(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "keys should not have matched")

	f.Flags["domain-1.kf-1"].Elements["key-1"] = nfdv1alpha1.Nil{}
	m, err = Execute(r2, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r2.Labels, m.Labels, "keys should have matched")
	assert.Equal(t, r2.Vars, m.Vars, "vars should be present")

	// Match "value" features
	r3 := &nfdv1alpha1.Rule{
		Labels: map[string]string{"label-3": "label-val-3", "empty": ""},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.vf-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"key-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "values should not have matched")

	f.Attributes["domain-1.vf-1"].Elements["key-1"] = "val-1"
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "values should have matched")

	// Match "instance" features
	r3 = &nfdv1alpha1.Rule{
		Labels: map[string]string{"label-4": "label-val-4"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "domain-1.if-1",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"attr-1": newMatchExpression(nfdv1alpha1.MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "instances should not have matched")

	f.Instances["domain-1.if-1"].Elements[0].Attributes["attr-1"] = "val-1"
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "instances should have matched")

	// Match "multi-type" features
	f2 := nfdv1alpha1.NewFeatures()
	f2.Flags["dom.feat"] = nfdv1alpha1.NewFlagFeatures("k-1", "k-2")
	f2.Attributes["dom.feat"] = nfdv1alpha1.NewAttributeFeatures(map[string]string{"a-1": "v-1", "a-2": "v-2"})
	f2.Instances["dom.feat"] = nfdv1alpha1.NewInstanceFeatures(
		*nfdv1alpha1.NewInstanceFeature(map[string]string{"ia-1": "iv-1"}),
		*nfdv1alpha1.NewInstanceFeature(map[string]string{"ia-2": "iv-2"}),
	)

	r3 = &nfdv1alpha1.Rule{
		Labels: map[string]string{"feat": "val-1"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature: "dom.feat",
				MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
					"k-1": newMatchExpression(nfdv1alpha1.MatchExists),
				},
			},
		},
	}
	m, err = Execute(r3, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "key in multi-type feature should have matched")

	r3.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"a-1": newMatchExpression(nfdv1alpha1.MatchIn, "v-1"),
			},
		},
	}
	m, err = Execute(r3, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "attribute in multi-type feature should have matched")

	r3.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"ia-1": newMatchExpression(nfdv1alpha1.MatchIn, "iv-1"),
			},
		},
	}
	m, err = Execute(r3, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "attribute in multi-type feature should have matched")

	r3.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"k-2": newMatchExpression(nfdv1alpha1.MatchExists),
				"a-2": newMatchExpression(nfdv1alpha1.MatchIn, "v-2"),
			},
		},
	}
	m, err = Execute(r3, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "features in multi-type feature should have matched flags and attributes")

	r3.MatchFeatures = nfdv1alpha1.FeatureMatcher{
		nfdv1alpha1.FeatureMatcherTerm{
			Feature: "dom.feat",
			MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
				"ia-2": newMatchExpression(nfdv1alpha1.MatchIn, "iv-2"),
			},
		},
	}
	m, err = Execute(r3, f2, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "features in multi-type feature should have matched instance")

	// Test multiple feature matchers
	r3 = &nfdv1alpha1.Rule{
		Labels: map[string]string{"label-5": "label-val-5"},
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
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "instances should not have matched")

	(*r3.MatchFeatures[0].MatchExpressions)["key-1"] = newMatchExpression(nfdv1alpha1.MatchIn, "val-1")
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "instances should have matched")

	// Test MatchAny
	r3.MatchAny = []nfdv1alpha1.MatchAnyElem{
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
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "instances should not have matched")

	r3.MatchAny = append(r3.MatchAny,
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
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "instances should have matched")
}

func TestTemplating(t *testing.T) {
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

	r1 := &nfdv1alpha1.Rule{
		Labels: map[string]string{"label-1": "label-val-1"},
		LabelsTemplate: `
label-1=will-be-overridden
label-2=
{{range .domain_1.kf_1}}kf-{{.Name}}=present
{{end}}
{{range .domain_1.vf_1}}vf-{{.Name}}=vf-{{.Value}}
{{end}}
{{range .domain_1.if_1}}if-{{index . "attr-1"}}_{{index . "attr-2"}}=present
{{end}}`,
		Vars: map[string]string{"var-1": "var-val-1"},
		VarsTemplate: `
var-1=value-will-be-overridden-by-vars
var-2=
{{range .domain_1.kf_1}}kf-{{.Name}}=true
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

	// test with empty MatchFeatures, but with MatchAny
	r3 := r1.DeepCopy()
	r3.MatchAny = []nfdv1alpha1.MatchAnyElem{{MatchFeatures: r3.MatchFeatures}}
	r3.MatchFeatures = nil

	expectedLabels := map[string]string{
		"label-1": "label-val-1",
		"label-2": "",
		// From kf_1 template
		"kf-key-a": "present",
		"kf-key-c": "present",
		"kf-foo":   "present",
		// From vf_1 template
		"vf-key-1": "vf-val-1",
		"vf-bar":   "vf-",
		// From if_1 template
		"if-1_val-2":       "present",
		"if-10_val-20":     "present",
		"if-1000_val-2000": "present",
	}
	expectedVars := map[string]string{
		"var-1": "var-val-1",
		"var-2": "",
		// From template
		"kf-key-a": "true",
		"kf-key-c": "true",
		"kf-foo":   "true",
	}

	m, err := Execute(r1, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedLabels, m.Labels, "instances should have matched")
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedLabels, m.Labels, "instances should have matched")
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	// Test "multi-type" feature
	r3 = &nfdv1alpha1.Rule{
		LabelsTemplate: `
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

	expectedLabels = map[string]string{
		"mf-key-a": "found",
		"mf-key-d": "found",
	}
	expectedVars = map[string]string{}
	m, err = Execute(r3, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedLabels, m.Labels, "instances should have matched")
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	//
	// Test error cases
	//
	r2 := &nfdv1alpha1.Rule{
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

	r2.LabelsTemplate = "foo=bar"
	m, err = Execute(r2, f, true)
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, m.Labels, "instances should have matched")
	assert.Empty(t, m.Vars)

	r2.LabelsTemplate = "foo"
	_, err = Execute(r2, f, true)
	assert.Error(t, err)

	r2.LabelsTemplate = "{{"
	_, err = Execute(r2, f, true)
	assert.Error(t, err)

	r2.LabelsTemplate = ""
	r2.VarsTemplate = "bar=baz"
	m, err = Execute(r2, f, true)
	assert.Nil(t, err)
	assert.Empty(t, m.Labels)
	assert.Equal(t, map[string]string{"bar": "baz"}, m.Vars, "instances should have matched")

	r2.VarsTemplate = "bar"
	_, err = Execute(r2, f, true)
	assert.Error(t, err)

	r2.VarsTemplate = "{{"
	_, err = Execute(r2, f, true)
	assert.Error(t, err)

	//
	// Test matchName
	//
	r4 := &nfdv1alpha1.Rule{
		LabelsTemplate: "{{range .domain_1.vf_1}}{{.Name}}={{.Value}}\n{{end}}",
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
	expectedLabels = map[string]string{
		"key-1": "val-1",
		"key-4": "val-4",
		"key-5": "",
	}

	m, err = Execute(r4, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedLabels, m.Labels, "instances should have matched")

	r4 = &nfdv1alpha1.Rule{
		Labels: map[string]string{"should-not-match": "true"},
		MatchFeatures: nfdv1alpha1.FeatureMatcher{
			nfdv1alpha1.FeatureMatcherTerm{
				Feature:   "domain_1.vf_1",
				MatchName: newMatchExpression(nfdv1alpha1.MatchIn, "key-not-exists"),
			},
		},
	}

	m, err = Execute(r4, f, true)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, map[string]string(nil), m.Labels, "instances should have matched")
}
