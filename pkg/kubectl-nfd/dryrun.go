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

package kubectlnfd

import (
	"fmt"
	"maps"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/nodefeaturerule"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/validate"
)

func processNodeFeatureGroup(nodeFeatureGroup nfdv1alpha1.NodeFeatureGroup, nodeFeature nfdv1alpha1.NodeFeatureSpec) []error {
	var errs []error

	for _, rule := range nodeFeatureGroup.Spec.Rules {
		fmt.Println("Processing rule: ", rule.Name)
		ruleOut, err := nodefeaturerule.ExecuteGroupRule(&rule, &nodeFeature.Features, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to process rule %q: %w", rule.Name, err))
			continue
		}
		if ruleOut.MatchStatus == nil || !ruleOut.MatchStatus.IsMatch {
			fmt.Printf("Rule %q did not match\n", rule.Name)
			continue
		}
		fmt.Printf("Rule %q matched\n", rule.Name)
		if len(ruleOut.Vars) > 0 {
			fmt.Println("***\tVars\t***")
			for k, v := range ruleOut.Vars {
				fmt.Printf("%s=%s\n", k, v)
			}
		}
	}

	return errs
}

func DryRun(resourcepath, nodefeaturepath string) []error {
	nfFile, err := os.ReadFile(nodefeaturepath)
	if err != nil {
		return []error{fmt.Errorf("error reading NodeFeature file: %w", err)}
	}
	nf := nfdv1alpha1.NodeFeature{}
	if err = yaml.Unmarshal(nfFile, &nf); err != nil {
		return []error{fmt.Errorf("error parsing NodeFeature: %w", err)}
	}

	t := parseRuleFile(resourcepath)
	switch o := t.(type) {
	case *nfdv1alpha1.NodeFeatureRule:
		return processNodeFeatureRule(*o, nf.Spec)
	case *nfdv1alpha1.NodeFeatureGroup:
		return processNodeFeatureGroup(*o, nf.Spec)
	default:
		return []error{fmt.Errorf("unsupported resource %v: must be NodeFeatureRule or NodeFeatureGroup", t)}
	}
}

func processNodeFeatureRule(nodeFeatureRule nfdv1alpha1.NodeFeatureRule, nodeFeature nfdv1alpha1.NodeFeatureSpec) []error {
	var errs []error
	var taints []corev1.Taint

	extendedResources := make(map[string]string)
	labels := make(map[string]string)
	annotations := make(map[string]string)

	for _, rule := range nodeFeatureRule.Spec.Rules {
		fmt.Println("Processing rule: ", rule.Name)
		ruleOut, err := nodefeaturerule.Execute(&rule, &nodeFeature.Features, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to process rule: %q - %w", rule.Name, err))
			continue
		}
		// taints
		taints = append(taints, ruleOut.Taints...)
		// labels
		for k, v := range ruleOut.Labels {
			// Dynamic Value
			if strings.HasPrefix(v, "@") {
				dvalue, err := getDynamicValue(v, &nodeFeature.Features)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to get dynamic value for label %q: %w", k, err))
					continue
				}
				labels[k] = dvalue
				continue
			}
			labels[k] = v
		}
		// extended resources
		for k, v := range ruleOut.ExtendedResources {
			// Dynamic Value
			if strings.HasPrefix(v, "@") {
				dvalue, err := getDynamicValue(v, &nodeFeature.Features)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to get dynamic value for extendedResource %q: %w", k, err))
					continue
				}
				extendedResources[k] = dvalue
				continue
			}
			extendedResources[k] = v
		}
		// annotations
		maps.Copy(annotations, ruleOut.Annotations)
	}

	if len(taints) > 0 {
		taintValidation := validate.Taints(taints)
		fmt.Println("***\tTaints\t***")
		for _, taint := range taints {
			fmt.Println(taint)
		}
		if len(taintValidation) > 0 {
			fmt.Println("\t-Validation errors-")
			for _, err := range taintValidation {
				fmt.Println(err)
			}
		}
	}

	if len(labels) > 0 {
		labelValidation := validate.Labels(labels)
		fmt.Println("***\tLabels\t***")
		for k, v := range labels {
			fmt.Printf("%s=%s\n", k, v)
		}
		if len(labelValidation) > 0 {
			fmt.Println("\t-Validation errors-")
			for _, err := range labelValidation {
				fmt.Println(err)
			}
		}
	}

	if len(extendedResources) > 0 {
		resourceValidation := processExtendedResources(extendedResources, nodeFeature)
		fmt.Println("***\tExtended Resources\t***")
		for k, v := range extendedResources {
			fmt.Printf("%s=%s\n", k, v)
		}
		if len(resourceValidation) > 0 {
			fmt.Println("\t-Validation errors-")
			for _, err := range resourceValidation {
				fmt.Println(err)
			}
		}
	}

	if len(annotations) > 0 {
		annotationsValidation := validate.Annotations(annotations)
		fmt.Println("***\tAnnotations\t***")
		for k, v := range annotations {
			fmt.Printf("%s=%s\n", k, v)
		}
		if len(annotationsValidation) > 0 {
			fmt.Println("\t-Validation errors-")
			for _, err := range annotationsValidation {
				fmt.Println(err)
			}
		}
	}

	return errs
}

func processExtendedResources(extendedResources map[string]string, nodeFeature nfdv1alpha1.NodeFeatureSpec) []error {
	var errs []error
	return append(errs, validate.ExtendedResources(extendedResources)...)
}

func getDynamicValue(value string, features *nfdv1alpha1.Features) (string, error) {
	// value is a string in the form of attribute.featureset.elements
	split := strings.SplitN(value[1:], ".", 3)
	if len(split) != 3 {
		return "", fmt.Errorf("value %s is not in the form of '@domain.feature.element'", value)
	}
	featureName := split[0] + "." + split[1]
	elementName := split[2]
	attrFeatureSet, ok := features.Attributes[featureName]
	if !ok {
		return "", fmt.Errorf("feature %s not found", featureName)
	}
	element, ok := attrFeatureSet.Elements[elementName]
	if !ok {
		return "", fmt.Errorf("element %s not found on feature %s", elementName, featureName)
	}
	return element, nil
}
