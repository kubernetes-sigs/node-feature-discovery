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
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
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
func ExecuteGroupRule(r *nfdv1alpha1.GroupRule, features *nfdv1alpha1.Features, failFast bool) (bool, error) {
	matched := false
	if len(r.MatchAny) > 0 {
		// Logical OR over the matchAny matchers
		for _, matcher := range r.MatchAny {
			if isMatch, matches, err := evaluateMatchAnyElem(&matcher, features, failFast); err != nil {
				return false, err
			} else if isMatch {
				matched = true
				klog.V(4).InfoS("matchAny matched", "ruleName", r.Name, "matchedFeatures", utils.DelayedDumper(matches))
				// there's no need to evaluate other matchers in MatchAny
				// One match is enough for MatchAny
				break
			}
		}
		if !matched {
			return false, nil
		}
	}

	if len(r.MatchFeatures) > 0 {
		if isMatch, _, err := evaluateFeatureMatcher(&r.MatchFeatures, features, failFast); err != nil {
			return false, err
		} else if !isMatch {
			klog.V(2).InfoS("rule did not match", "ruleName", r.Name)
			return false, nil
		}
	}

	klog.V(2).InfoS("rule matched", "ruleName", r.Name)
	return true, nil
}

func executeLabelsTemplate(r *nfdv1alpha1.Rule, in matchedFeatures, out map[string]string) error {
	if r.LabelsTemplate == "" {
		return nil
	}

	th, err := newTemplateHelper(r.LabelsTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse LabelsTemplate: %w", err)
	}

	labels, err := th.expandMap(in)
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

	th, err := newTemplateHelper(r.VarsTemplate)
	if err != nil {
		return err
	}

	vars, err := th.expandMap(in)
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
			return false, nil, fmt.Errorf("feature %q not available", featureName)
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

type templateHelper struct {
	template *template.Template
}

func newTemplateHelper(name string) (*templateHelper, error) {
	tmpl, err := template.New("").Option("missingkey=error").Parse(name)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	return &templateHelper{template: tmpl}, nil
}

func (h *templateHelper) execute(data interface{}) (string, error) {
	var tmp bytes.Buffer
	if err := h.template.Execute(&tmp, data); err != nil {
		return "", err
	}
	return tmp.String(), nil
}

// expandMap is a helper for expanding a template in to a map of strings. Data
// after executing the template is expexted to be key=value pairs separated by
// newlines.
func (h *templateHelper) expandMap(data interface{}) (map[string]string, error) {
	expanded, err := h.execute(data)
	if err != nil {
		return nil, err
	}

	// Split out individual key-value pairs
	out := make(map[string]string)
	for _, item := range strings.Split(expanded, "\n") {
		// Remove leading/trailing whitespace and skip empty lines
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			split := strings.SplitN(trimmed, "=", 2)
			if len(split) == 1 {
				return nil, fmt.Errorf("missing value in expanded template line %q, (format must be '<key>=<value>')", trimmed)
			}
			out[split[0]] = split[1]
		}
	}
	return out, nil
}
