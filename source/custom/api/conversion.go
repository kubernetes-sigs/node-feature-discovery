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
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
)

// convertFeaturematchertermToV1alpha1 converts the internal api type to nfdv1alpha1.
func convertFeaturematchertermToV1alpha1(in *FeatureMatcherTerm, out *nfdv1alpha1.FeatureMatcherTerm) error {
	out.Feature = in.Feature
	if in.MatchExpressions != nil {
		inME := in.MatchExpressions
		outME := make(nfdv1alpha1.MatchExpressionSet, len(*inME))
		for key := range *inME {
			me := &nfdv1alpha1.MatchExpression{}
			if err := convertMatchexpressionToV1alpha1((*inME)[key], me); err != nil {
				return err
			}
			outME[key] = me
		}
		out.MatchExpressions = &outME
	} else {
		out.MatchExpressions = nil
	}

	if in.MatchName != nil {
		out.MatchName = &nfdv1alpha1.MatchExpression{}
		if err := convertMatchexpressionToV1alpha1(in.MatchName, out.MatchName); err != nil {
			return err
		}
	} else {
		out.MatchName = nil
	}
	return nil
}

// convertMatchanyelemToV1alpha1 converts the internal api type to nfdv1alpha1.
func convertMatchanyelemToV1alpha1(in *MatchAnyElem, out *nfdv1alpha1.MatchAnyElem) error {
	if in.MatchFeatures != nil {
		inMF, outMF := &in.MatchFeatures, &out.MatchFeatures
		*outMF = make(nfdv1alpha1.FeatureMatcher, len(*inMF))
		for i := range *inMF {
			if err := convertFeaturematchertermToV1alpha1(&(*inMF)[i], &(*outMF)[i]); err != nil {
				return err
			}
		}
	} else {
		out.MatchFeatures = nil
	}
	return nil
}

// convertMatchexpressionToV1alpha1 converts the internal api type to nfdv1alpha1.
func convertMatchexpressionToV1alpha1(in *MatchExpression, out *nfdv1alpha1.MatchExpression) error {
	out.Op = nfdv1alpha1.MatchOp(in.Op)
	if in.Value != nil {
		in, out := &in.Value, &out.Value
		*out = make(nfdv1alpha1.MatchValue, len(*in))
		copy(*out, *in)
	} else {
		out.Value = nil
	}
	return nil
}

// ConvertRuleToV1alpha1 converts the internal api type to nfdv1alpha1.
func ConvertRuleToV1alpha1(in *Rule, out *nfdv1alpha1.Rule) error {
	out.Name = in.Name
	out.Labels = in.Labels
	out.LabelsTemplate = in.LabelsTemplate
	out.Vars = in.Vars
	out.VarsTemplate = in.VarsTemplate
	if in.MatchFeatures != nil {
		in, out := &in.MatchFeatures, &out.MatchFeatures
		*out = make(nfdv1alpha1.FeatureMatcher, len(*in))
		for i := range *in {
			if err := convertFeaturematchertermToV1alpha1(&(*in)[i], &(*out)[i]); err != nil {
				return err
			}
		}
	} else {
		out.MatchFeatures = nil
	}
	if in.MatchAny != nil {
		in, out := &in.MatchAny, &out.MatchAny
		*out = make([]nfdv1alpha1.MatchAnyElem, len(*in))
		for i := range *in {
			if err := convertMatchanyelemToV1alpha1(&(*in)[i], &(*out)[i]); err != nil {
				return err
			}
		}
	} else {
		out.MatchAny = nil
	}
	return nil
}
