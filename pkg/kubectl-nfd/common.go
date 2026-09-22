/*
Copyright 2026 The Kubernetes Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func parseRuleFile(filepath string) (obj interface{}) {
	file, err := os.ReadFile(filepath)
	if err != nil {
		return []error{fmt.Errorf("error reading file: %w", err)}
	}
	typeMeta := metav1.TypeMeta{}
	if err = yaml.Unmarshal(file, &typeMeta); err != nil {
		return []error{fmt.Errorf("error reading resource kind: %w", err)}
	}
	switch typeMeta.Kind {
	case "NodeFeatureRule":
		nfr := nfdv1alpha1.NodeFeatureRule{}
		if err := yaml.Unmarshal(file, &nfr); err != nil {
			return []error{fmt.Errorf("error reading NodeFeatureRule file: %w", err)}
		}
		return &nfr
	case "NodeFeatureGroup":
		nfg := nfdv1alpha1.NodeFeatureGroup{}
		if err := yaml.Unmarshal(file, &nfg); err != nil {
			return []error{fmt.Errorf("error reading NodeFeatureGroup file: %w", err)}
		}
		return &nfg
	default:
		return []error{fmt.Errorf("unsupported resource kind %q: must be NodeFeatureRule or NodeFeatureGroup", typeMeta.Kind)}
	}
}
