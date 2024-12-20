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
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	compatv1alpha1 "sigs.k8s.io/node-feature-discovery/api/image-compatibility/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	artifactcli "sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/artifact-client"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/fake"
)

func init() {
	fs := source.GetConfigurableSource(fake.Name)
	fs.SetConfig(fs.NewConfig())
}

func TestNodeValidator(t *testing.T) {
	ctx := context.Background()

	Convey("With a single compatibility set that contains flags, attributes and instances", t, func() {
		spec := &compatv1alpha1.Spec{
			Version: compatv1alpha1.Version,
			Compatibilties: []compatv1alpha1.Compatibility{
				{
					Description: "Fake compatibility",
					Rules: []v1alpha1.Rule{
						{
							Name: "fake_1",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature:   "fake.flag",
									MatchName: &v1alpha1.MatchExpression{Op: v1alpha1.MatchInRegexp, Value: v1alpha1.MatchValue{"^flag"}},
								},
							},
						},
						{
							Name: "fake_2",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature: "fake.flag",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"flag_unknown": &v1alpha1.MatchExpression{Op: v1alpha1.MatchExists},
									},
								},
								{
									Feature: "fake.attribute",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"attr_1": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"true"}},
									},
								},
							},
						},
						{
							Name: "fake_3",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature: "fake.instance",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"name":   &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_1"}},
										"attr_1": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"true"}},
									},
								},
								{
									Feature: "fake.instance",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"name":   &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_2"}},
										"attr_1": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
									},
								},
							},
						},
						{
							Name: "fake_4",
							MatchAny: []v1alpha1.MatchAnyElem{
								{
									MatchFeatures: v1alpha1.FeatureMatcher{
										{
											Feature: "fake.instance",
											MatchExpressions: &v1alpha1.MatchExpressionSet{
												"name": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_1"}},
											},
										},
									},
								},
								{
									MatchFeatures: v1alpha1.FeatureMatcher{
										{
											Feature: "fake.instance",
											MatchExpressions: &v1alpha1.MatchExpressionSet{
												"name": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_unknown"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		// The output contains expressions in alphabetical order over the feature, name and expression string.
		expectedOutput := []*CompatibilityStatus{
			{
				Description: "Fake compatibility",
				Rules: []ProcessedRuleStatus{
					{
						Name:    "fake_1",
						IsMatch: true,
						MatchedExpressions: []MatchedExpression{
							{
								Feature:     "fake.flag",
								Name:        "",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchInRegexp, Value: v1alpha1.MatchValue{"^flag"}},
								MatcherType: MatchNameType,
								IsMatch:     true,
							},
						},
					},
					{
						Name:    "fake_2",
						IsMatch: false,
						MatchedExpressions: []MatchedExpression{
							{
								Feature:     "fake.attribute",
								Name:        "attr_1",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"true"}},
								MatcherType: MatchExpressionType,
								IsMatch:     true,
							},
							{
								Feature:     "fake.flag",
								Name:        "flag_unknown",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchExists},
								MatcherType: MatchExpressionType,
								IsMatch:     false,
							},
						},
					},
					{
						Name:    "fake_3",
						IsMatch: false,
						MatchedExpressions: []MatchedExpression{
							{
								Feature:     "fake.instance",
								Name:        "attr_1",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
								MatcherType: MatchExpressionType,
								IsMatch:     false,
							},
							{
								Feature:     "fake.instance",
								Name:        "attr_1",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"true"}},
								MatcherType: MatchExpressionType,
								IsMatch:     true,
							},
							{
								Feature:     "fake.instance",
								Name:        "name",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_1"}},
								MatcherType: MatchExpressionType,
								IsMatch:     true,
							},
							{
								Feature:     "fake.instance",
								Name:        "name",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_2"}},
								MatcherType: MatchExpressionType,
								IsMatch:     true,
							},
						},
					},
					{
						Name:    "fake_4",
						IsMatch: true,
						MatchedAny: []MatchAnyElem{
							{
								MatchedExpressions: []MatchedExpression{
									{
										Feature:     "fake.instance",
										Name:        "name",
										Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_1"}},
										MatcherType: MatchExpressionType,
										IsMatch:     true,
									},
								},
							},
							{
								MatchedExpressions: []MatchedExpression{
									{
										Feature:     "fake.instance",
										Name:        "name",
										Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"instance_unknown"}},
										MatcherType: MatchExpressionType,
										IsMatch:     false,
									},
								},
							},
						},
					},
				},
			},
		}

		validator := New(
			WithArgs(&Args{}),
			WithArtifactClient(newMock(ctx, spec)),
			WithSources(map[string]source.FeatureSource{fake.Name: source.GetFeatureSource(fake.Name)}),
		)
		output, err := validator.Execute(ctx)

		So(err, ShouldBeNil)
		So(output, ShouldEqual, expectedOutput)
	})

	Convey("With multiple compatibility sets", t, func() {
		spec := &compatv1alpha1.Spec{
			Version: compatv1alpha1.Version,
			Compatibilties: []compatv1alpha1.Compatibility{
				{
					Tag:         "prefered",
					Weight:      90,
					Description: "Fake compatibility 1",
					Rules: []v1alpha1.Rule{
						{
							Name: "fake_1",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature: "fake.attribute",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"attr_1": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
									},
								},
							},
						},
					},
				},
				{
					Tag:         "fallback",
					Weight:      40,
					Description: "Fake compatibility 2",
					Rules: []v1alpha1.Rule{
						{
							Name: "fake_1",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature: "fake.attribute",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"attr_2": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
									},
								},
							},
						},
					},
				},
			},
		}

		expectedOutput := []*CompatibilityStatus{
			{
				Tag:         "prefered",
				Weight:      90,
				Description: "Fake compatibility 1",
				Rules: []ProcessedRuleStatus{
					{
						Name:    "fake_1",
						IsMatch: false,
						MatchedExpressions: []MatchedExpression{
							{
								Feature:     "fake.attribute",
								Name:        "attr_1",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
								MatcherType: MatchExpressionType,
								IsMatch:     false,
							},
						},
					},
				},
			},
			{
				Tag:         "fallback",
				Weight:      40,
				Description: "Fake compatibility 2",
				Rules: []ProcessedRuleStatus{
					{
						Name:    "fake_1",
						IsMatch: true,
						MatchedExpressions: []MatchedExpression{
							{
								Feature:     "fake.attribute",
								Name:        "attr_2",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
								MatcherType: MatchExpressionType,
								IsMatch:     true,
							},
						},
					},
				},
			},
		}

		validator := New(
			WithArgs(&Args{}),
			WithArtifactClient(newMock(ctx, spec)),
			WithSources(map[string]source.FeatureSource{fake.Name: source.GetFeatureSource(fake.Name)}),
		)
		output, err := validator.Execute(ctx)

		So(err, ShouldBeNil)
		So(output, ShouldEqual, expectedOutput)
	})

	Convey("With compatibility sets filtered out by tags", t, func() {
		spec := &compatv1alpha1.Spec{
			Version: compatv1alpha1.Version,
			Compatibilties: []compatv1alpha1.Compatibility{
				{
					Tag:         "prefered",
					Weight:      90,
					Description: "Fake compatibility 1",
					Rules: []v1alpha1.Rule{
						{
							Name: "fake_1",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature: "fake.attribute",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"attr_1": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
									},
								},
							},
						},
					},
				},
				{
					Tag:         "fallback",
					Weight:      40,
					Description: "Fake compatibility 2",
					Rules: []v1alpha1.Rule{
						{
							Name: "fake_1",
							MatchFeatures: v1alpha1.FeatureMatcher{
								{
									Feature: "fake.attribute",
									MatchExpressions: &v1alpha1.MatchExpressionSet{
										"attr_2": &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
									},
								},
							},
						},
					},
				},
			},
		}

		expectedOutput := []*CompatibilityStatus{
			{
				Tag:         "prefered",
				Weight:      90,
				Description: "Fake compatibility 1",
				Rules: []ProcessedRuleStatus{
					{
						Name:    "fake_1",
						IsMatch: false,
						MatchedExpressions: []MatchedExpression{
							{
								Feature:     "fake.attribute",
								Name:        "attr_1",
								Expression:  &v1alpha1.MatchExpression{Op: v1alpha1.MatchIn, Value: v1alpha1.MatchValue{"false"}},
								MatcherType: MatchExpressionType,
								IsMatch:     false,
							},
						},
					},
				},
			},
		}

		validator := New(
			WithArgs(&Args{
				Tags: []string{"prefered"},
			}),
			WithArtifactClient(newMock(ctx, spec)),
			WithSources(map[string]source.FeatureSource{fake.Name: source.GetFeatureSource(fake.Name)}),
		)
		output, err := validator.Execute(ctx)

		So(err, ShouldBeNil)
		So(output, ShouldEqual, expectedOutput)
	})
}

func newMock(ctx context.Context, result *compatv1alpha1.Spec) *artifactcli.MockArtifactClient {
	artifactClient := &artifactcli.MockArtifactClient{}
	artifactClient.On("FetchCompatibilitySpec", ctx).Return(result, nil)
	return artifactClient
}
