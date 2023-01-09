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

package topologypolicy

import (
	"testing"

	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
)

func TestDetectTopologyPolicy(t *testing.T) {
	testCases := []struct {
		scope    string
		policy   string
		expected v1alpha1.TopologyManagerPolicy
	}{
		{
			policy:   "best-effort",
			scope:    "pod",
			expected: v1alpha1.BestEffortPodLevel,
		},
		{
			policy:   "best-effort",
			scope:    "container",
			expected: v1alpha1.BestEffortContainerLevel,
		},
		{
			policy:   "restricted",
			scope:    "container",
			expected: v1alpha1.RestrictedContainerLevel,
		},
		{
			policy:   "restricted",
			scope:    "pod",
			expected: v1alpha1.RestrictedPodLevel,
		},
		{
			policy:   "single-numa-node",
			scope:    "pod",
			expected: v1alpha1.SingleNUMANodePodLevel,
		},
		{
			policy:   "single-numa-node",
			scope:    "container",
			expected: v1alpha1.SingleNUMANodeContainerLevel,
		},
		{
			policy:   "none",
			scope:    "container",
			expected: v1alpha1.None,
		},
		{
			policy:   "none",
			scope:    "pod",
			expected: v1alpha1.None,
		},
		{
			policy:   "non-existent",
			scope:    "pod",
			expected: v1alpha1.None,
		},
		{
			policy:   "non-existent",
			scope:    "container",
			expected: v1alpha1.None,
		},
		{
			policy:   "single-numa-node",
			scope:    "non-existent",
			expected: v1alpha1.None,
		},
		{
			policy:   "single-numa-node",
			scope:    "non-existent",
			expected: v1alpha1.None,
		},
	}

	for _, tc := range testCases {
		actual := DetectTopologyPolicy(tc.policy, tc.scope)
		if actual != tc.expected {
			t.Errorf("Expected TopologyPolicy to equal: %s not: %s", tc.expected, actual)
		}
	}
}
