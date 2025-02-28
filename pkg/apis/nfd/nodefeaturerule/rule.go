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
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/template"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

// MatchStatus represents the status of a processed rule.
// It includes information about successful expressions and their results, which are the matched host features.
// For example, for the expression: cpu.cpuid: {op: "InRegexp", value: ["^AVX"]},
// the result could include matched host features such as AVX, AVX2, AVX512 etc.
// +k8s:deepcopy-gen=false
type MatchStatus struct {
	*MatchFeatureStatus

	// IsMatch informes whether a rule succeeded or failed.
	IsMatch bool
	// MatchAny represents an array of logical OR conditions between MatchFeatureStatus entries.
	MatchAny []*MatchFeatureStatus
}

// MatchFeatureStatus represents a matched expression
// with its result, which is matched host features.
// +k8s:deepcopy-gen=false
type MatchFeatureStatus struct {
	// MatchedFeatures represents the features matched on the host,
	// which is a result of the FeatureMatcher.
	MatchedFeatures matchedFeatures
	// MatchedFeaturesTerms represents the expressions that successfully matched on the host.
	MatchedFeaturesTerms nfdv1alpha1.FeatureMatcher
}

// RuleOutput contains the output out rule execution.
// +k8s:deepcopy-gen=false
type RuleOutput struct {
	ExtendedResources map[string]string
	Labels            map[string]string
	Annotations       map[string]string
	Vars              map[string]string
	Taints            []corev1.Taint
	MatchStatus       *MatchStatus
}

// Execute the rule against a set of input features.
func Execute(r *nfdv1alpha1.Rule, features *nfdv1alpha1.Features, failFast bool) (RuleOutput, error) {
	var (
		matchStatus MatchStatus
		isMatch     bool
		err         error
	)
	labels := make(map[string]string)
	vars := make(map[string]string)

	if n := len(r.MatchAny); n > 0 {
		matchStatus.MatchAny = make([]*MatchFeatureStatus, 0, n)
		// Logical OR over the matchAny matchers
		var (
			featureStatus *MatchFeatureStatus
			matched       bool
		)
		for _, matcher := range r.MatchAny {
			if matched, featureStatus, err = evaluateMatchAnyElem(&matcher, features, failFast); err != nil {
				return RuleOutput{}, err
			} else if matched {
				isMatch = true
				klog.V(4).InfoS("matchAny matched", "ruleName", r.Name, "matchedFeatures", utils.DelayedDumper(featureStatus.MatchedFeatures))

				if r.LabelsTemplate == "" && r.VarsTemplate == "" && failFast {
					// there's no need to evaluate other matchers in MatchAny
					// if there are no templates to be executed on them - so
					// short-circuit and stop on first match here
					break
				}

				if err := executeLabelsTemplate(r, featureStatus.MatchedFeatures, labels); err != nil {
					return RuleOutput{}, err
				}
				if err := executeVarsTemplate(r, featureStatus.MatchedFeatures, vars); err != nil {
					return RuleOutput{}, err
				}
			}

			matchStatus.MatchAny = append(matchStatus.MatchAny, featureStatus)
		}

		if !isMatch {
			klog.V(2).InfoS("rule did not match", "ruleName", r.Name)
			return RuleOutput{MatchStatus: &matchStatus}, nil
		}
	}

	if len(r.MatchFeatures) > 0 {
		if isMatch, matchStatus.MatchFeatureStatus, err = evaluateFeatureMatcher(&r.MatchFeatures, features, failFast); err != nil {
			return RuleOutput{}, err
		} else if !isMatch {
			klog.V(2).InfoS("rule did not match", "ruleName", r.Name)
			return RuleOutput{MatchStatus: &matchStatus}, nil
		} else {
			klog.V(4).InfoS("matchFeatures matched", "ruleName", r.Name, "matchedFeatures", utils.DelayedDumper(matchStatus.MatchedFeatures))
			if err := executeLabelsTemplate(r, matchStatus.MatchedFeatures, labels); err != nil {
				return RuleOutput{}, err
			}
			if err := executeVarsTemplate(r, matchStatus.MatchedFeatures, vars); err != nil {
				return RuleOutput{}, err
			}
		}
	}

	maps.Copy(labels, r.Labels)
	maps.Copy(vars, r.Vars)
	matchStatus.IsMatch = true

	ret := RuleOutput{
		Labels:            labels,
		Vars:              vars,
		Annotations:       maps.Clone(r.Annotations),
		ExtendedResources: maps.Clone(r.ExtendedResources),
		Taints:            slices.Clone(r.Taints),
		MatchStatus:       &matchStatus,
	}
	klog.V(2).InfoS("rule matched", "ruleName", r.Name, "ruleOutput", utils.DelayedDumper(ret))
	return ret, nil
}

// ExecuteGroupRule executes the GroupRule against a set of input features, and return true if the
// rule matches.
func ExecuteGroupRule(r *nfdv1alpha1.GroupRule, features *nfdv1alpha1.Features, failFast bool) (MatchStatus, error) {
	var (
		matchStatus MatchStatus
		isMatch     bool
	)
	if n := len(r.MatchAny); n > 0 {
		matchStatus.MatchAny = make([]*MatchFeatureStatus, 0, n)
		// Logical OR over the matchAny matchers
		for _, matcher := range r.MatchAny {
			matched, featureStatus, err := evaluateMatchAnyElem(&matcher, features, failFast)
			if err != nil {
				return matchStatus, err
			} else if matched {
				isMatch = true
				klog.V(4).InfoS("matchAny matched", "ruleName", r.Name, "matchedFeatures", utils.DelayedDumper(featureStatus.MatchedFeatures))

				if failFast {
					// there's no need to evaluate other matchers in MatchAny
					break
				}
			}
			matchStatus.MatchAny = append(matchStatus.MatchAny, featureStatus)
		}
		if !isMatch && failFast {
			return matchStatus, nil
		}
	}

	if len(r.MatchFeatures) > 0 {
		var err error
		if isMatch, matchStatus.MatchFeatureStatus, err = evaluateFeatureMatcher(&r.MatchFeatures, features, failFast); err != nil {
			return matchStatus, err
		} else if !isMatch {
			klog.V(2).InfoS("rule did not match", "ruleName", r.Name)
			return matchStatus, nil
		}
	}

	matchStatus.IsMatch = true

	klog.V(2).InfoS("rule matched", "ruleName", r.Name)
	return matchStatus, nil
}

func executeLabelsTemplate(r *nfdv1alpha1.Rule, in matchedFeatures, out map[string]string) error {
	if r.LabelsTemplate == "" {
		return nil
	}

	th, err := template.NewHelper(r.LabelsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse LabelsTemplate: %w", err)
	}

	labels, err := th.ExpandMap(in)
	if err != nil {
		return fmt.Errorf("failed to expand LabelsTemplate: %w", err)
	}
	for k, v := range labels {
		out[k] = v
	}
	return nil
}

func executeVarsTemplate(r *nfdv1alpha1.Rule, in matchedFeatures, out map[string]string) error {
	if r.VarsTemplate == "" {
		return nil
	}

	th, err := template.NewHelper(r.VarsTemplate)
	if err != nil {
		return err
	}

	vars, err := th.ExpandMap(in)
	if err != nil {
		return err
	}
	for k, v := range vars {
		out[k] = v
	}
	return nil
}

type matchedFeatures map[string]domainMatchedFeatures

type domainMatchedFeatures map[string][]MatchedElement

func evaluateMatchAnyElem(e *nfdv1alpha1.MatchAnyElem, features *nfdv1alpha1.Features, failFast bool) (bool, *MatchFeatureStatus, error) {
	return evaluateFeatureMatcher(&e.MatchFeatures, features, failFast)
}

func evaluateFeatureMatcher(m *nfdv1alpha1.FeatureMatcher, features *nfdv1alpha1.Features, failFast bool) (bool, *MatchFeatureStatus, error) {
	var (
		isMatch     = true
		isTermMatch = true
	)
	status := &MatchFeatureStatus{
		MatchedFeatures: make(matchedFeatures, len(*m)),
	}

	// Logical AND over the terms
	for _, term := range *m {
		// Ignore case
		featureName := strings.ToLower(term.Feature)

		nameSplit := strings.SplitN(term.Feature, ".", 2)
		if len(nameSplit) != 2 {
			klog.InfoS("invalid feature name (not <domain>.<feature>), cannot be used for templating", "featureName", term.Feature)
			nameSplit = []string{featureName, ""}
		}

		dom := nameSplit[0]
		nam := nameSplit[1]
		if _, ok := status.MatchedFeatures[dom]; !ok {
			status.MatchedFeatures[dom] = make(domainMatchedFeatures)
		}

		var matchedElems []MatchedElement
		var matchedExpressions *nfdv1alpha1.MatchExpressionSet
		var err error

		matchedFeatureTerm := nfdv1alpha1.FeatureMatcherTerm{
			Feature: featureName,
		}
		fF, okF := features.Flags[featureName]
		fA, okA := features.Attributes[featureName]
		fI, okI := features.Instances[featureName]
		if !okF && !okA && !okI {
			klog.V(2).InfoS("feature not available", "featureName", featureName)
			if failFast {
				return false, nil, nil
			}
			isMatch = false
			continue
		}

		if term.MatchExpressions != nil {
			isTermMatch, matchedElems, matchedExpressions, err = MatchMulti(term.MatchExpressions, fF.Elements, fA.Elements, fI.Elements, failFast)
			matchedFeatureTerm.MatchExpressions = matchedExpressions
		}

		if err == nil && isTermMatch && term.MatchName != nil {
			var meTmp []MatchedElement
			isTermMatch, meTmp, err = MatchNamesMulti(term.MatchName, fF.Elements, fA.Elements, fI.Elements)
			matchedElems = append(matchedElems, meTmp...)
			// MatchName has only one expression, in this case it's enough to check the isTermMatch flag
			// to judge if the expression succeeded on the host.
			if isTermMatch {
				matchedFeatureTerm.MatchName = term.MatchName
			}
		}

		status.MatchedFeatures[dom][nam] = append(status.MatchedFeatures[dom][nam], matchedElems...)
		if matchedFeatureTerm.MatchName != nil || (matchedFeatureTerm.MatchExpressions != nil && len(*matchedFeatureTerm.MatchExpressions) > 0) {
			status.MatchedFeaturesTerms = append(status.MatchedFeaturesTerms, matchedFeatureTerm)
		}

		if err != nil {
			return false, nil, err
		} else if !isTermMatch {
			if !failFast {
				isMatch = false
			} else {
				return false, status, nil
			}
		}
	}
	return isMatch, status, nil
}
