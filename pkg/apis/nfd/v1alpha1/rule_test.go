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
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
)

func TestRule(t *testing.T) {
	f := map[string]*feature.DomainFeatures{}
	r1 := Rule{Labels: map[string]string{"label-1": "", "label-2": "true"}}
	r2 := Rule{
		Labels:      map[string]string{"label-1": "label-val-1"},
		Annotations: map[string]string{"annotation-1": "annotation-val-1"},
		Vars:        map[string]string{"var-1": "var-val-1"},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature: "domain-1.kf-1",
				MatchExpressions: MatchExpressionSet{
					"key-1": MustCreateMatchExpression(MatchExists),
				},
			},
		},
	}

	// Test totally empty features
	m, err := r1.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")

	_, err = r2.Execute(f)
	assert.Error(t, err, "matching against a missing domain should have returned an error")

	// Test empty domain
	d := feature.NewDomainFeatures()
	f["domain-1"] = d

	m, err = r1.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")
	assert.Empty(t, r1.Vars, "vars should be empty")
	assert.Empty(t, r1.Annotations, "annotations should be empty")

	_, err = r2.Execute(f)
	assert.Error(t, err, "matching against a missing feature type should have returned an error")

	// Test empty feature sets
	d.Keys["kf-1"] = feature.NewKeyFeatures()
	d.Values["vf-1"] = feature.NewValueFeatures(nil)
	d.Instances["if-1"] = feature.NewInstanceFeatures(nil)

	m, err = r1.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")

	m, err = r2.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "unexpected match")

	// Test non-empty feature sets
	d.Keys["kf-1"].Elements["key-x"] = feature.Nil{}
	d.Values["vf-1"].Elements["key-1"] = "val-x"
	d.Instances["if-1"] = feature.NewInstanceFeatures([]feature.InstanceFeature{
		*feature.NewInstanceFeature(map[string]string{"attr-1": "val-x"})})

	m, err = r1.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty matcher should have matched empty features")

	// Test empty MatchExpressions
	r1.MatchFeatures = FeatureMatcher{
		FeatureMatcherTerm{
			Feature:          "domain-1.kf-1",
			MatchExpressions: MatchExpressionSet{},
		},
	}
	m, err = r1.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r1.Labels, m.Labels, "empty match expression set mathces anything")

	// Match "key" features
	m, err = r2.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "keys should not have matched")

	d.Keys["kf-1"].Elements["key-1"] = feature.Nil{}
	m, err = r2.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r2.Labels, m.Labels, "keys should have matched")
	assert.Equal(t, r2.Annotations, m.Annotations, "annotations should be present")
	assert.Equal(t, r2.Vars, m.Vars, "vars should be present")

	// Match "value" features
	r3 := Rule{
		Labels: map[string]string{"label-3": "label-val-3", "empty": ""},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature: "domain-1.vf-1",
				MatchExpressions: MatchExpressionSet{
					"key-1": MustCreateMatchExpression(MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = r3.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "values should not have matched")

	d.Values["vf-1"].Elements["key-1"] = "val-1"
	m, err = r3.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r3.Labels, m.Labels, "values should have matched")

	// Match "instance" features
	r4 := Rule{
		Labels: map[string]string{"label-4": "label-val-4"},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature: "domain-1.if-1",
				MatchExpressions: MatchExpressionSet{
					"attr-1": MustCreateMatchExpression(MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = r4.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "instances should not have matched")

	d.Instances["if-1"].Elements[0].Attributes["attr-1"] = "val-1"
	m, err = r4.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r4.Labels, m.Labels, "instances should have matched")

	// Test multiple feature matchers
	r5 := Rule{
		Labels: map[string]string{"label-5": "label-val-5"},
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature: "domain-1.vf-1",
				MatchExpressions: MatchExpressionSet{
					"key-1": MustCreateMatchExpression(MatchIn, "val-x"),
				},
			},
			FeatureMatcherTerm{
				Feature: "domain-1.if-1",
				MatchExpressions: MatchExpressionSet{
					"attr-1": MustCreateMatchExpression(MatchIn, "val-1"),
				},
			},
		},
	}
	m, err = r5.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "instances should not have matched")

	r5.MatchFeatures[0].MatchExpressions["key-1"] = MustCreateMatchExpression(MatchIn, "val-1")
	m, err = r5.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Labels, m.Labels, "instances should have matched")

	// Test MatchAny
	r5.MatchAny = []MatchAnyElem{
		MatchAnyElem{
			MatchFeatures: FeatureMatcher{
				FeatureMatcherTerm{
					Feature: "domain-1.kf-1",
					MatchExpressions: MatchExpressionSet{
						"key-na": MustCreateMatchExpression(MatchExists),
					},
				},
			},
		},
	}
	m, err = r5.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Nil(t, m.Labels, "instances should not have matched")

	r5.MatchAny = append(r5.MatchAny,
		MatchAnyElem{
			MatchFeatures: FeatureMatcher{
				FeatureMatcherTerm{
					Feature: "domain-1.kf-1",
					MatchExpressions: MatchExpressionSet{
						"key-1": MustCreateMatchExpression(MatchExists),
					},
				},
			},
		})
	r5.MatchFeatures[0].MatchExpressions["key-1"] = MustCreateMatchExpression(MatchIn, "val-1")
	m, err = r5.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, r5.Labels, m.Labels, "instances should have matched")
}

func TestTemplating(t *testing.T) {
	f := map[string]*feature.DomainFeatures{
		"domain_1": &feature.DomainFeatures{
			Keys: map[string]feature.KeyFeatureSet{
				"kf_1": feature.KeyFeatureSet{
					Elements: map[string]feature.Nil{
						"key-a": feature.Nil{},
						"key-b": feature.Nil{},
						"key-c": feature.Nil{},
					},
				},
			},
			Values: map[string]feature.ValueFeatureSet{
				"vf_1": feature.ValueFeatureSet{
					Elements: map[string]string{
						"key-1": "val-1",
						"keu-2": "val-2",
						"key-3": "val-3",
					},
				},
			},
			Instances: map[string]feature.InstanceFeatureSet{
				"if_1": feature.InstanceFeatureSet{
					Elements: []feature.InstanceFeature{
						feature.InstanceFeature{
							Attributes: map[string]string{
								"attr-1": "1",
								"attr-2": "val-2",
							},
						},
						feature.InstanceFeature{
							Attributes: map[string]string{
								"attr-1": "10",
								"attr-2": "val-20",
							},
						},
						feature.InstanceFeature{
							Attributes: map[string]string{
								"attr-1": "100",
								"attr-2": "val-200",
							},
						},
					},
				},
			},
		},
	}

	r1 := Rule{
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
		Annotations: map[string]string{"annotation-1": "annotation-val-1"},
		AnnotationsTemplate: `
annotation-1=will-be-overriden
annotation-2=
kf-1={{range .domain_1.kf_1}}{{.Name}} {{end}}`,
		Vars: map[string]string{"var-1": "var-val-1"},
		VarsTemplate: `
var-1=value-will-be-overridden-by-vars
var-2=
{{range .domain_1.kf_1}}kf-{{.Name}}=true
{{end}}`,
		MatchFeatures: FeatureMatcher{
			FeatureMatcherTerm{
				Feature: "domain_1.kf_1",
				MatchExpressions: MatchExpressionSet{
					"key-a": MustCreateMatchExpression(MatchExists),
					"key-c": MustCreateMatchExpression(MatchExists),
					"foo":   MustCreateMatchExpression(MatchDoesNotExist),
				},
			},
			FeatureMatcherTerm{
				Feature: "domain_1.vf_1",
				MatchExpressions: MatchExpressionSet{
					"key-1": MustCreateMatchExpression(MatchIn, "val-1", "val-2"),
					"bar":   MustCreateMatchExpression(MatchDoesNotExist),
				},
			},
			FeatureMatcherTerm{
				Feature: "domain_1.if_1",
				MatchExpressions: MatchExpressionSet{
					"attr-1": MustCreateMatchExpression(MatchLt, "100"),
				},
			},
		},
	}

	// test with empty MatchFeatures, but with MatchAny
	r3 := r1.DeepCopy()
	r3.MatchAny = []MatchAnyElem{{MatchFeatures: r3.MatchFeatures}}
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
		"if-1_val-2":   "present",
		"if-10_val-20": "present",
	}
	expectedAnnotations := map[string]string{
		"annotation-1": "annotation-val-1",
		"annotation-2": "",
		// From template
		"kf-1": "foo key-a key-c",
	}
	expectedVars := map[string]string{
		"var-1": "var-val-1",
		"var-2": "",
		// From template
		"kf-key-a": "true",
		"kf-key-c": "true",
		"kf-foo":   "true",
	}

	m, err := r1.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedLabels, m.Labels, "instances should have matched")
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")
	assert.Equal(t, expectedAnnotations, m.Annotations, "instances should have matched")

	m, err = r3.Execute(f)
	assert.Nilf(t, err, "unexpected error: %v", err)
	assert.Equal(t, expectedLabels, m.Labels, "instances should have matched")
	assert.Equal(t, expectedVars, m.Vars, "instances should have matched")

	//
	// Test error cases
	//
	r2 := Rule{
		MatchFeatures: FeatureMatcher{
			// We need at least one matcher to match to execute the template.
			// Use a simple empty matchexpression set to match anything.
			FeatureMatcherTerm{
				Feature: "domain_1.kf_1",
				MatchExpressions: MatchExpressionSet{
					"key-a": MustCreateMatchExpression(MatchExists),
				},
			},
		},
	}

	r2.LabelsTemplate = "foo=bar"
	m, err = r2.Execute(f)
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, m.Labels, "instances should have matched")
	assert.Empty(t, m.Annotations)
	assert.Empty(t, m.Vars)

	r2.labelsTemplate = nil
	r2.LabelsTemplate = "foo"
	_, err = r2.Execute(f)
	assert.Error(t, err)

	r2.labelsTemplate = nil
	r2.LabelsTemplate = "{{"
	_, err = r2.Execute(f)
	assert.Error(t, err)

	r2.labelsTemplate = nil
	r2.LabelsTemplate = ""

	r2.AnnotationsTemplate = "boo=far"
	m, err = r2.Execute(f)
	assert.Nil(t, err)
	assert.Empty(t, m.Labels)
	assert.Equal(t, map[string]string{"boo": "far"}, m.Annotations, "instances should have matched")
	assert.Empty(t, m.Vars)

	r2.annotationsTemplate = nil
	r2.AnnotationsTemplate = "boo"
	_, err = r2.Execute(f)
	assert.Error(t, err)

	r2.annotationsTemplate = nil
	r2.AnnotationsTemplate = "{{"
	_, err = r2.Execute(f)
	assert.Error(t, err)

	r2.annotationsTemplate = nil
	r2.AnnotationsTemplate = ""

	r2.VarsTemplate = "bar=baz"
	m, err = r2.Execute(f)
	assert.Nil(t, err)
	assert.Empty(t, m.Labels)
	assert.Empty(t, m.Annotations)
	assert.Equal(t, map[string]string{"bar": "baz"}, m.Vars, "instances should have matched")

	r2.varsTemplate = nil
	r2.VarsTemplate = "bar"
	_, err = r2.Execute(f)
	assert.Error(t, err)

	r2.varsTemplate = nil
	r2.VarsTemplate = "{{"
	_, err = r2.Execute(f)
	assert.Error(t, err)

}
