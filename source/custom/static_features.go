/*
Copyright 2020 The Kubernetes Authors.

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

package custom

import (
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
)

// getStaticFeatures returns statically configured custom features to discover
// e.g RMDA related features. NFD configuration file may extend these custom features by adding rules.
func getStaticFeatureConfig() []CustomRule {
	return []CustomRule{
		{
			Rule: &Rule{
				nfdv1alpha1.Rule{
					Name:   "RDMA capable static rule",
					Labels: map[string]string{"rdma.capable": "true"},
					MatchFeatures: nfdv1alpha1.FeatureMatcher{
						nfdv1alpha1.FeatureMatcherTerm{
							Feature: "pci.device",
							MatchExpressions: nfdv1alpha1.MatchExpressionSet{
								"vendor": nfdv1alpha1.MustCreateMatchExpression(nfdv1alpha1.MatchIn, "15b3"),
							},
						},
					},
				},
			},
		},
		{
			Rule: &Rule{
				nfdv1alpha1.Rule{
					Name:   "RDMA available static rule",
					Labels: map[string]string{"rdma.available": "true"},
					MatchFeatures: nfdv1alpha1.FeatureMatcher{
						nfdv1alpha1.FeatureMatcherTerm{
							Feature: "kernel.loadedmodule",
							MatchExpressions: nfdv1alpha1.MatchExpressionSet{
								"ib_uverbs": nfdv1alpha1.MustCreateMatchExpression(nfdv1alpha1.MatchExists),
								"rdma_ucm":  nfdv1alpha1.MustCreateMatchExpression(nfdv1alpha1.MatchExists),
							},
						},
					},
				},
			},
		},
	}

}
