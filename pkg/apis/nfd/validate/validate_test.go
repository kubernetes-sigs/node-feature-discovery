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
package validate

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  interface{}
	}{
		{
			name:  "Valid annotation",
			key:   "feature.node.kubernetes.io/feature",
			value: "true",
			want:  nil,
		},
		{
			name:  "Invalid annotation key",
			key:   "_invalid-key_",
			value: "true",
			want:  "invalid annotation key \"_invalid-key_\":",
		},
		{
			name:  "Denied annotation key",
			key:   "denied-key",
			value: "true",
			want:  ErrUnprefixedKeysNotAllowed,
		},
		{
			name:  "Invalid annotation value",
			key:   "feature.node.kubernetes.io/feature",
			value: string(make([]byte, 1100)),
			want:  "invalid value: too long:",
		},
		{
			name:  "Denied annotation key",
			key:   "kubernetes.io/denied",
			value: "true",
			want:  ErrNSNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Annotation(tt.key, tt.value)
			if str, ok := tt.want.(string); ok {
				assert.ErrorContains(t, err, str)
			} else {
				assert.Equal(t, tt.want, err)
			}
		})
	}
}

func TestAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        []error
	}{
		{
			name:        "Empty annotations",
			annotations: map[string]string{},
			want:        nil,
		},
		{
			name: "Valid annotations",
			annotations: map[string]string{
				"feature.node.kubernetes.io/annotation": "true",
				"vendor.io/annotation":                  "true",
			},
			want: nil,
		},
		{
			name: "Invalid annotations",
			annotations: map[string]string{
				"invalid-key":              "true",
				"kubernetes.io/annotation": "true",
			},
			want: []error{
				ErrUnprefixedKeysNotAllowed,
				ErrNSNotAllowed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := sortErrors(Annotations(tt.annotations))
			assert.Equal(t, len(tt.want), len(errs))
			for i := range errs {
				assert.ErrorIs(t, errs[i], tt.want[i])
			}
		})
	}
}

func TestTaint(t *testing.T) {
	tests := []struct {
		name  string
		taint *corev1.Taint
		want  error
	}{
		{
			name: "Valid taint",
			taint: &corev1.Taint{
				Key:    "feature.node.kubernetes.io/taint",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: nil,
		},
		{
			name: "UNPREFIXED taint key",
			taint: &corev1.Taint{
				Key:    "invalid-key",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: ErrUnprefixedKeysNotAllowed,
		},
		{
			name: "Invalid taint key",
			taint: &corev1.Taint{
				Key:    "invalid.kubernetes.io/invalid-key",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: ErrNSNotAllowed,
		},
		{
			name: "Empty taint effect",
			taint: &corev1.Taint{
				Key:    "feature.node.kubernetes.io/taint",
				Value:  "true",
				Effect: "",
			},
			want: ErrEmptyTaintEffect,
		},
		{
			name: "Invalid taint effect",
			taint: &corev1.Taint{
				Key:    "feature.node.kubernetes.io/taint",
				Value:  "true",
				Effect: "invalid-effect",
			},
			want: ErrInvalidTaintEffect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Taint(tt.taint)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTaints(t *testing.T) {
	tests := []struct {
		name   string
		taints []corev1.Taint
		want   []error
	}{
		{
			name:   "Empty taints",
			taints: []corev1.Taint{},
			want:   nil,
		},
		{
			name: "Valid taints",
			taints: []corev1.Taint{
				{
					Key:    "feature.node.kubernetes.io/taint",
					Value:  "true",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "vendor.io/taint",
					Value:  "true",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
			want: nil,
		},
		{
			name: "Invalid taints",
			taints: []corev1.Taint{
				{
					Key:    "invalid-key",
					Value:  "true",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "feature.node.kubernetes.io/taint",
					Value:  "true",
					Effect: "",
				},
				{
					Key:    "feature.node.kubernetes.io/taint",
					Value:  "true",
					Effect: "invalid-effect",
				},
			},
			want: []error{
				ErrUnprefixedKeysNotAllowed,
				ErrEmptyTaintEffect,
				ErrInvalidTaintEffect,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Taints(tt.taints)
			assert.Equal(t, len(tt.want), len(errs))
			for i := range errs {
				assert.ErrorIs(t, errs[i], tt.want[i])
			}
		})
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  interface{}
	}{
		{
			name:  "Valid label",
			key:   "feature.node.kubernetes.io/label",
			value: "true",
			want:  nil,
		},
		{
			name:  "Valid vendor label",
			key:   "vendor.io/label",
			value: "true",
			want:  nil,
		},
		{
			name:  "Invalid label key",
			key:   "invalid-key:",
			value: "true",
			want:  "invalid label key \"invalid-key:\": ",
		},
		{
			name:  "Denied label with prefix",
			key:   "kubernetes.io/label",
			value: "true",
			want:  ErrNSNotAllowed,
		},
		{
			name:  "Denied label key unprefixed",
			key:   "denied-key",
			value: "true",
			want:  ErrUnprefixedKeysNotAllowed,
		},
		{
			name:  "Invalid label value",
			key:   "feature.node.kubernetes.io/label",
			value: "invalid value",
			want:  "invalid value \"invalid value\": ",
		},
		{
			name:  "Valid value label",
			key:   "feature.node.kubernetes.io/label",
			value: "true",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Label(tt.key, tt.value)
			if str, ok := tt.want.(string); ok {
				assert.ErrorContains(t, err, str)
			} else {
				assert.Equal(t, tt.want, err)
			}
		})
	}
}

func TestLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   []error
	}{
		{
			name:   "Empty labels",
			labels: map[string]string{},
			want:   nil,
		},
		{
			name: "Valid labels",
			labels: map[string]string{
				"feature.node.kubernetes.io/label": "true",
				"vendor.io/label":                  "true",
			},
			want: nil,
		},
		{
			name: "Invalid labels",
			labels: map[string]string{
				"invalid-key":         "true",
				"kubernetes.io/label": "true",
			},
			want: []error{
				ErrUnprefixedKeysNotAllowed,
				ErrNSNotAllowed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sortErrors(Labels(tt.labels))
			assert.Equal(t, len(tt.want), len(err))
			for i := range err {
				assert.ErrorIs(t, err[i], tt.want[i])
			}
		})
	}
}

func TestExtendedResource(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  interface{}
	}{
		{
			name:  "Valid extended resource",
			key:   "feature.node.kubernetes.io/extended-resource",
			value: "123",
			want:  nil,
		},
		{
			name:  "Invalid extended resource name",
			key:   "invalid-name~",
			value: "123",
			want:  "invalid name \"invalid-name~\": ",
		},
		{
			name:  "Denied extended resource key",
			key:   "denied-key",
			value: "123",
			want:  ErrUnprefixedKeysNotAllowed,
		},
		{
			name:  "Invalid extended resource value",
			key:   "feature.node.kubernetes.io/extended-resource",
			value: "invalid value",
			want:  "invalid value \"invalid value\": ",
		},
		{
			name:  "Denied extended resource key",
			key:   "kubernetes.io/extended-resource",
			value: "123",
			want:  ErrNSNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtendedResource(tt.key, tt.value)
			if str, ok := tt.want.(string); ok {
				assert.ErrorContains(t, err, str)
			} else {
				assert.Equal(t, tt.want, err)
			}
		})
	}
}

func TestExtendedResources(t *testing.T) {
	tests := []struct {
		name              string
		extendedResources map[string]string
		want              []error
	}{
		{
			name:              "Empty extended resources",
			extendedResources: map[string]string{},
			want:              nil,
		},
		{
			name: "Valid extended resources",
			extendedResources: map[string]string{
				"feature.node.kubernetes.io/extended-resource": "123",
				"vendor.io/extended-resource":                  "456",
			},
			want: nil,
		},
		{
			name: "Invalid extended resources",
			extendedResources: map[string]string{
				"invalid-key":                     "456",
				"kubernetes.io/extended-resource": "123",
			},
			want: []error{
				ErrUnprefixedKeysNotAllowed,
				ErrNSNotAllowed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := sortErrors(ExtendedResources(tt.extendedResources))
			assert.Equal(t, len(tt.want), len(errs))
			for i := range errs {
				assert.ErrorIs(t, errs[i], tt.want[i])
			}
		})
	}
}

func TestMatchFeatures(t *testing.T) {
	tests := []struct {
		name           string
		matchFeature   nfdv1alpha1.FeatureMatcher
		expectedErrors []error
	}{
		{
			name:           "Empty matchFeature",
			matchFeature:   nfdv1alpha1.FeatureMatcher{},
			expectedErrors: nil,
		},
		{
			name: "Valid matchFeature",
			matchFeature: nfdv1alpha1.FeatureMatcher{
				{
					Feature: "domain1.feature1",
				},
				{
					Feature: "domain2.feature2",
				},
			},
			expectedErrors: nil,
		},
		{
			name: "Invalid matchFeature",
			matchFeature: nfdv1alpha1.FeatureMatcher{
				{
					Feature: "invalid-feature",
				},
				{
					Feature: "domain3",
				},
				{
					Feature: "prefix.domain.feature",
				},
			},
			expectedErrors: []error{
				fmt.Errorf("invalid feature name invalid-feature (not <domain>.<feature>), cannot be used for templating"),
				fmt.Errorf("invalid feature name domain3 (not <domain>.<feature>), cannot be used for templating"),
				fmt.Errorf("invalid feature name prefix.domain.feature (not <domain>.<feature>), cannot be used for templating"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := MatchFeatures(tt.matchFeature)
			assert.Equal(t, tt.expectedErrors, errors)
		})
	}
}

func TestMatchAny(t *testing.T) {
	tests := []struct {
		name           string
		matchAny       []nfdv1alpha1.MatchAnyElem
		expectedErrors []error
	}{
		{
			name:           "Empty matchAny",
			matchAny:       []nfdv1alpha1.MatchAnyElem{},
			expectedErrors: nil,
		},
		{
			name: "Valid matchAny",
			matchAny: []nfdv1alpha1.MatchAnyElem{
				{
					MatchFeatures: nfdv1alpha1.FeatureMatcher{
						{
							Feature: "domain1.feature1",
						},
						{
							Feature: "domain2.feature2",
						},
					},
				},
				{
					MatchFeatures: nfdv1alpha1.FeatureMatcher{
						{
							Feature: "domain3.feature3",
						},
						{
							Feature: "domain4.feature4",
						},
					},
				},
			},
			expectedErrors: nil,
		},
		{
			name: "Invalid matchAny",
			matchAny: []nfdv1alpha1.MatchAnyElem{
				{
					MatchFeatures: nfdv1alpha1.FeatureMatcher{
						{
							Feature: "invalid-feature",
						},
						{
							Feature: "domain3",
						},
					},
				},
				{
					MatchFeatures: nfdv1alpha1.FeatureMatcher{
						{
							Feature: "domain5.feature5",
						},
						{
							Feature: "invalid.domain.feature6",
						},
					},
				},
			},
			expectedErrors: []error{
				fmt.Errorf("invalid feature name invalid-feature (not <domain>.<feature>), cannot be used for templating"),
				fmt.Errorf("invalid feature name domain3 (not <domain>.<feature>), cannot be used for templating"),
				fmt.Errorf("invalid feature name invalid.domain.feature6 (not <domain>.<feature>), cannot be used for templating"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := MatchAny(tt.matchAny)
			assert.Equal(t, tt.expectedErrors, errors)
		})
	}
}

func TestTemplate(t *testing.T) {
	tests := []struct {
		name           string
		labelsTemplate string
		want           []error
	}{
		{
			name:           "Valid template",
			labelsTemplate: "key1=value1,key2=value2",
			want:           nil,
		},
		{
			name:           "Invalid template",
			labelsTemplate: "{{.key1=value1,key2=value2}}",
			want:           []error{fmt.Errorf("invalid template: template: :1: bad character U+003D '='")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Template(tt.labelsTemplate)
			assert.Equal(t, len(tt.want), len(errs))
			for i := range errs {
				assert.EqualError(t, errs[i], tt.want[i].Error())
			}
		})
	}
}

func sortErrors(errs []error) []error {
	sort.Slice(errs, func(i, j int) bool {
		return errs[i].Error() < errs[j].Error()
	})
	return errs
}
