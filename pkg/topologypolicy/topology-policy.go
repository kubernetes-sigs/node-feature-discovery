package topologypolicy

// DetectTopologyPolicy returns TopologyManagerPolicy type which present
// both Topology manager policy and scope
func DetectTopologyPolicy(policy string, scope string) TopologyManagerPolicy {
	k8sTmPolicy := K8sTopologyManagerPolicies(policy)
	k8sTmScope := K8sTopologyManagerScopes(scope)
	switch k8sTmPolicy {
	case singleNumaNode:
		if k8sTmScope == pod {
			return SingleNumaPodScope
		}
		// default scope for single-numa-node
		return SingleNumaContainerScope
	case restricted:
		return Restricted
	case bestEffort:
		return BestEffort
	default:
		return None
	}
}
