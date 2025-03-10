/*
Copyright 2025 The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	gentype "k8s.io/client-go/gentype"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/typed/nfd/v1alpha1"
	v1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

// fakeNodeFeatureRules implements NodeFeatureRuleInterface
type fakeNodeFeatureRules struct {
	*gentype.FakeClientWithList[*v1alpha1.NodeFeatureRule, *v1alpha1.NodeFeatureRuleList]
	Fake *FakeNfdV1alpha1
}

func newFakeNodeFeatureRules(fake *FakeNfdV1alpha1) nfdv1alpha1.NodeFeatureRuleInterface {
	return &fakeNodeFeatureRules{
		gentype.NewFakeClientWithList[*v1alpha1.NodeFeatureRule, *v1alpha1.NodeFeatureRuleList](
			fake.Fake,
			"",
			v1alpha1.SchemeGroupVersion.WithResource("nodefeaturerules"),
			v1alpha1.SchemeGroupVersion.WithKind("NodeFeatureRule"),
			func() *v1alpha1.NodeFeatureRule { return &v1alpha1.NodeFeatureRule{} },
			func() *v1alpha1.NodeFeatureRuleList { return &v1alpha1.NodeFeatureRuleList{} },
			func(dst, src *v1alpha1.NodeFeatureRuleList) { dst.ListMeta = src.ListMeta },
			func(list *v1alpha1.NodeFeatureRuleList) []*v1alpha1.NodeFeatureRule {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1alpha1.NodeFeatureRuleList, items []*v1alpha1.NodeFeatureRule) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
