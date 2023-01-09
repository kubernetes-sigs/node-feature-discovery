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

package topologypolicy

import (
	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/apis/config"
)

// DetectTopologyPolicy returns string type which present
// both Topology manager policy and scope
func DetectTopologyPolicy(policy string, scope string) v1alpha1.TopologyManagerPolicy {
	switch scope {
	case config.PodTopologyManagerScope:
		return detectPolicyPodScope(policy)
	case config.ContainerTopologyManagerScope:
		return detectPolicyContainerScope(policy)
	default:
		return v1alpha1.None
	}
}

func detectPolicyPodScope(policy string) v1alpha1.TopologyManagerPolicy {
	switch policy {
	case config.SingleNumaNodeTopologyManagerPolicy:
		return v1alpha1.SingleNUMANodePodLevel
	case config.RestrictedTopologyManagerPolicy:
		return v1alpha1.RestrictedPodLevel
	case config.BestEffortTopologyManagerPolicy:
		return v1alpha1.BestEffortPodLevel
	case config.NoneTopologyManagerPolicy:
		return v1alpha1.None
	default:
		return v1alpha1.None
	}
}

func detectPolicyContainerScope(policy string) v1alpha1.TopologyManagerPolicy {
	switch policy {
	case config.SingleNumaNodeTopologyManagerPolicy:
		return v1alpha1.SingleNUMANodeContainerLevel
	case config.RestrictedTopologyManagerPolicy:
		return v1alpha1.RestrictedContainerLevel
	case config.BestEffortTopologyManagerPolicy:
		return v1alpha1.BestEffortContainerLevel
	case config.NoneTopologyManagerPolicy:
		return v1alpha1.None
	default:
		return v1alpha1.None
	}
}
