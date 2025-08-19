/*
Copyright 2023 The Kubernetes Authors.

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

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestRuleConversion(t *testing.T) {
	type TC struct {
		name     string
		internal Rule
		external nfdv1alpha1.Rule
	}
	tcs := []TC{
		{
			name:     "empty rule",
			internal: Rule{},
			external: nfdv1alpha1.Rule{},
		},
		{
			name: "all fields populated",
			internal: Rule{
				Name: "test rule 1",
				Labels: map[string]string{
					"label-1": "val-1",
					"label-2": "val-2",
				},
				LabelsTemplate: "{{ range .fake.attribute }}example.com/fake-{{ .Name }}={{ .Value }}\n{{ end }}",
				Vars: map[string]string{
					"var-a": "val-a",
					"var-b": "val-b",
				},
				VarsTemplate: "{{ range .fake.attribute }}fake-{{ .Name }}={{ .Value }}\n{{ end }}",
				MatchFeatures: FeatureMatcher{
					FeatureMatcherTerm{
						Feature: "fake.attribute",
						MatchExpressions: &MatchExpressionSet{
							"attr_1": &MatchExpression{Op: MatchIn, Value: MatchValue{"true"}},
							"attr_2": &MatchExpression{Op: MatchInRegexp, Value: MatchValue{"^f"}},
						},
						MatchName: &MatchExpression{Op: MatchIn, Value: MatchValue{"elem-1"}},
					},
				},
				MatchAny: []MatchAnyElem{
					{
						MatchFeatures: FeatureMatcher{
							FeatureMatcherTerm{
								Feature: "fake.instance",
								MatchExpressions: &MatchExpressionSet{
									"name": &MatchExpression{Op: MatchNotIn, Value: MatchValue{"instance_1"}},
								},
								MatchName: &MatchExpression{Op: MatchIn, Value: MatchValue{"elem-2"}},
							},
						},
					},
				},
			},
			external: nfdv1alpha1.Rule{
				Name: "test rule 1",
				Labels: map[string]string{
					"label-1": "val-1",
					"label-2": "val-2",
				},
				LabelsTemplate: "{{ range .fake.attribute }}example.com/fake-{{ .Name }}={{ .Value }}\n{{ end }}",
				Vars: map[string]string{
					"var-a": "val-a",
					"var-b": "val-b",
				},
				VarsTemplate: "{{ range .fake.attribute }}fake-{{ .Name }}={{ .Value }}\n{{ end }}",
				MatchFeatures: nfdv1alpha1.FeatureMatcher{
					nfdv1alpha1.FeatureMatcherTerm{
						Feature: "fake.attribute",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
							"attr_1": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"true"}},
							"attr_2": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchInRegexp, Value: nfdv1alpha1.MatchValue{"^f"}},
						},
						MatchName: &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"elem-1"}},
					},
				},
				MatchAny: []nfdv1alpha1.MatchAnyElem{
					{
						MatchFeatures: nfdv1alpha1.FeatureMatcher{
							nfdv1alpha1.FeatureMatcherTerm{
								Feature: "fake.instance",
								MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
									"name": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchNotIn, Value: nfdv1alpha1.MatchValue{"instance_1"}},
								},
								MatchName: &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"elem-2"}},
							},
						},
					},
				},
			},
		},
		{
			name: "matchName is nil",
			internal: Rule{
				Name: "test rule 1",
				MatchFeatures: FeatureMatcher{
					FeatureMatcherTerm{
						Feature: "fake.attribute",
						MatchExpressions: &MatchExpressionSet{
							"attr_1": &MatchExpression{Op: MatchIsTrue},
						},
						MatchName: nil,
					},
				},
			},
			external: nfdv1alpha1.Rule{
				Name: "test rule 1",
				MatchFeatures: nfdv1alpha1.FeatureMatcher{
					nfdv1alpha1.FeatureMatcherTerm{
						Feature: "fake.attribute",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
							"attr_1": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIsTrue},
						},
						MatchName: nil,
					},
				},
			},
		},
		{
			name: "matchExpressions is empty",
			internal: Rule{
				Name: "test rule 1",
				MatchFeatures: FeatureMatcher{
					FeatureMatcherTerm{
						Feature:          "fake.attribute",
						MatchExpressions: &MatchExpressionSet{},
					},
				},
			},
			external: nfdv1alpha1.Rule{
				Name: "test rule 1",
				MatchFeatures: nfdv1alpha1.FeatureMatcher{
					nfdv1alpha1.FeatureMatcherTerm{
						Feature:          "fake.attribute",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{},
					},
				},
			},
		},
		{
			name: "matchExpressions is nil",
			internal: Rule{
				Name: "test rule 1",
				MatchFeatures: FeatureMatcher{
					FeatureMatcherTerm{
						Feature:          "fake.attribute",
						MatchExpressions: nil,
						MatchName:        &MatchExpression{Op: MatchInRegexp, Value: MatchValue{"^elem-"}},
					},
				},
			},
			external: nfdv1alpha1.Rule{
				Name: "test rule 1",
				MatchFeatures: nfdv1alpha1.FeatureMatcher{
					nfdv1alpha1.FeatureMatcherTerm{
						Feature:          "fake.attribute",
						MatchExpressions: nil,
						MatchName:        &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchInRegexp, Value: nfdv1alpha1.MatchValue{"^elem-"}},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := nfdv1alpha1.Rule{}
			err := ConvertRuleToV1alpha1(&tc.internal, &out)
			assert.Nil(t, err)
			assert.Equal(t, tc.external, out)
		})
	}
}
