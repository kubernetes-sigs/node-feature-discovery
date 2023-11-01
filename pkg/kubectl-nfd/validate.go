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
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/validate"
)

// Given a file path, read the file and check if is a valid NodeFeatureRule file
func ValidateNFR(filepath string) []error {
	var err error
	var validationErr []error

	file, err := os.ReadFile(filepath)
	if err != nil {
		return []error{fmt.Errorf("error reading NodeFeatureRule file: %w", err)}
	}

	nfr := nfdv1alpha1.NodeFeatureRule{}
	err = yaml.Unmarshal(file, &nfr)
	if err != nil {
		return []error{fmt.Errorf("error reading NodeFeatureRule file: %w", err)}
	}

	for _, rule := range nfr.Spec.Rules {
		fmt.Println("Validating rule: ", rule.Name)
		// Validate Rule Name
		if rule.Name == "" {
			validationErr = append(validationErr, fmt.Errorf("rule name cannot be empty"))
		}

		// Validate Annotations
		validationErr = append(validationErr, validate.Annotations(rule.Annotations)...)

		// Validate labels
		// Dummy dynamic values before validating labels
		labels := rule.Labels
		for k, v := range labels {
			if strings.HasPrefix(v, "@") {
				labels[k] = resource.NewQuantity(0, resource.DecimalSI).String()
			}
		}
		validationErr = append(validationErr, validate.Labels(labels)...)

		// Validate Taints
		validationErr = append(validationErr, validate.Taints(rule.Taints)...)

		// Validate extended Resources
		// Dummy dynamic values before validating extended resources
		extendedResources := rule.ExtendedResources
		for k, v := range extendedResources {
			if strings.HasPrefix(v, "@") {
				extendedResources[k] = resource.NewQuantity(0, resource.DecimalSI).String()
			}
		}
		validationErr = append(validationErr, validate.ExtendedResources(extendedResources)...)

		// Validate LabelsTemplate
		validationErr = append(validationErr, validate.Template(rule.LabelsTemplate)...)

		// Validate VarsTemplate
		validationErr = append(validationErr, validate.Template(rule.VarsTemplate)...)

		// Validate matchFeatures
		validationErr = append(validationErr, validate.MatchFeatures(rule.MatchFeatures)...)

		// Validate matchAny
		validationErr = append(validationErr, validate.MatchAny(rule.MatchAny)...)
	}

	return validationErr
}
