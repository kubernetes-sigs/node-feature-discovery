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
	"fmt"
	"slices"
	"sort"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/nodefeaturerule"
	artifactcli "sigs.k8s.io/node-feature-discovery/pkg/client-nfd/compat/artifact-client"
	"sigs.k8s.io/node-feature-discovery/source"

	// register sources
	_ "sigs.k8s.io/node-feature-discovery/source/cpu"
	_ "sigs.k8s.io/node-feature-discovery/source/kernel"
	_ "sigs.k8s.io/node-feature-discovery/source/memory"
	_ "sigs.k8s.io/node-feature-discovery/source/network"
	_ "sigs.k8s.io/node-feature-discovery/source/pci"
	_ "sigs.k8s.io/node-feature-discovery/source/storage"
	_ "sigs.k8s.io/node-feature-discovery/source/system"
	_ "sigs.k8s.io/node-feature-discovery/source/usb"
)

// Args holds command line arguments.
type Args struct {
	Tags []string
}

type nodeValidator struct {
	args Args

	artifactClient artifactcli.ArtifactClient
	sources        map[string]source.FeatureSource
}

// New builds a node validator with specified options.
func New(opts ...NodeValidatorOpts) nodeValidator {
	n := nodeValidator{}
	for _, opt := range opts {
		opt.apply(&n)
	}
	return n
}

// Execute pulls the compatibility artifact to compare described features with the ones discovered on the node.
func (nv *nodeValidator) Execute(ctx context.Context) ([]*CompatibilityStatus, error) {
	spec, err := nv.artifactClient.FetchCompatibilitySpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch compatibility spec: %w", err)
	}

	for _, s := range nv.sources {
		if err := s.Discover(); err != nil {
			return nil, fmt.Errorf("error during discovery of source %s: %w", s.Name(), err)
		}
	}
	features := source.GetAllFeatures()

	compats := []*CompatibilityStatus{}
	for _, c := range spec.Compatibilties {
		if len(nv.args.Tags) > 0 && !slices.Contains(nv.args.Tags, c.Tag) {
			continue
		}
		compat := newCompatibilityStatus(&c)

		for _, r := range c.Rules {
			ruleOut, err := nodefeaturerule.Execute(&r, features, false)
			if err != nil {
				return nil, err
			}
			compat.Rules = append(compat.Rules, nv.evaluateRuleStatus(&r, ruleOut.MatchStatus))

			// Add the 'rule.matched' feature for backreference functionality
			features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Labels)
			features.InsertAttributeFeatures(nfdv1alpha1.RuleBackrefDomain, nfdv1alpha1.RuleBackrefFeature, ruleOut.Vars)
		}
		compats = append(compats, &compat)
	}

	return compats, nil
}

func (nv *nodeValidator) evaluateRuleStatus(rule *nfdv1alpha1.Rule, matchStatus *nodefeaturerule.MatchStatus) ProcessedRuleStatus {
	out := ProcessedRuleStatus{Name: rule.Name, IsMatch: matchStatus.IsMatch}

	matchedFeatureTerms := nfdv1alpha1.FeatureMatcher{}
	if m := matchStatus.MatchFeatureStatus; m != nil {
		matchedFeatureTerms = m.MatchedFeaturesTerms
	}
	out.MatchedExpressions = nv.matchFeatureExpressions(rule.MatchFeatures, matchedFeatureTerms)

	for i, matchAnyElem := range rule.MatchAny {
		matchedFeatureTermsAny := nfdv1alpha1.FeatureMatcher{}
		if t := matchStatus.MatchAny[i].MatchedFeaturesTerms; t != nil {
			matchedFeatureTermsAny = t
		}
		matchedExpressions := nv.matchFeatureExpressions(matchAnyElem.MatchFeatures, matchedFeatureTermsAny)
		out.MatchedAny = append(out.MatchedAny, MatchAnyElem{MatchedExpressions: matchedExpressions})
	}

	return out
}

func (nv *nodeValidator) matchFeatureExpressions(featureMatcher, matchedFeatureTerms nfdv1alpha1.FeatureMatcher) []MatchedExpression {
	var out []MatchedExpression

	for _, term := range featureMatcher {
		if term.MatchExpressions != nil {
			out = append(out, nv.matchExpressions(term, matchedFeatureTerms)...)
		}
		if term.MatchName != nil {
			out = append(out, nv.matchName(term, matchedFeatureTerms))
		}
	}

	// For reproducible output sort by name, feature, expression.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Feature != out[j].Feature {
			return out[i].Feature < out[j].Feature
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Expression.String() < out[j].Expression.String()
	})

	return out
}

func (nodeValidator) matchExpressions(term nfdv1alpha1.FeatureMatcherTerm, matchedFeatureTerms nfdv1alpha1.FeatureMatcher) []MatchedExpression {
	var out []MatchedExpression

	for name, exp := range *term.MatchExpressions {
		isMatch := false
		for _, processedTerm := range matchedFeatureTerms {
			if term.Feature != processedTerm.Feature || processedTerm.MatchExpressions == nil {
				continue
			}
			pexp, ok := (*processedTerm.MatchExpressions)[name]
			if isMatch = ok && exp.Op == pexp.Op && slices.Equal(exp.Value, pexp.Value); isMatch {
				break
			}
		}

		out = append(out, MatchedExpression{
			Feature:     term.Feature,
			Name:        name,
			Expression:  exp,
			MatcherType: MatchExpressionType,
			IsMatch:     isMatch,
		})
	}

	return out
}

func (nodeValidator) matchName(term nfdv1alpha1.FeatureMatcherTerm, matchedFeatureTerms nfdv1alpha1.FeatureMatcher) MatchedExpression {
	isMatch := false
	for _, processedTerm := range matchedFeatureTerms {
		if term.Feature != processedTerm.Feature || processedTerm.MatchName == nil {
			continue
		}
		isMatch = term.MatchName.Op == processedTerm.MatchName.Op && slices.Equal(term.MatchName.Value, processedTerm.MatchName.Value)
		if isMatch {
			break
		}
	}
	return MatchedExpression{
		Feature:     term.Feature,
		Name:        "",
		Expression:  term.MatchName,
		MatcherType: MatchNameType,
		IsMatch:     isMatch,
	}
}

// NodeValidatorOpts applies certain options to the node validator.
type NodeValidatorOpts interface {
	apply(*nodeValidator)
}

type nodeValidatorOpt struct {
	f func(*nodeValidator)
}

func (o *nodeValidatorOpt) apply(nv *nodeValidator) {
	o.f(nv)
}

// WithArgs applies command line arguments to the node validator object.
func WithArgs(args *Args) NodeValidatorOpts {
	return &nodeValidatorOpt{f: func(nv *nodeValidator) { nv.args = *args }}
}

// WithArtifactClient applies the client for all artifact operations.
func WithArtifactClient(cli artifactcli.ArtifactClient) NodeValidatorOpts {
	return &nodeValidatorOpt{f: func(nv *nodeValidator) { nv.artifactClient = cli }}
}

// WithSources applies the list of enabled feature sources.
func WithSources(sources map[string]source.FeatureSource) NodeValidatorOpts {
	return &nodeValidatorOpt{f: func(nv *nodeValidator) { nv.sources = sources }}
}
