package resourcemonitor

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

const (
	cpu             = string(corev1.ResourceCPU)
	memory          = string(corev1.ResourceMemory)
	hugepages2Mi    = "hugepages-2Mi"
	nicResourceName = "vendor/nic1"
)

func TestNewExcludeResourceList(t *testing.T) {
	tests := []struct {
		desc                      string
		excludeListConfig         map[string][]string
		nodeName                  string
		expectedExcludedResources []string
	}{
		{

			desc: "exclude list with multiple nodes",
			excludeListConfig: map[string][]string{
				"node1": {
					cpu,
					nicResourceName,
				},
				"node2": {
					memory,
					hugepages2Mi,
				},
			},
			nodeName:                  "node1",
			expectedExcludedResources: []string{cpu, nicResourceName},
		},
		{
			desc: "exclude list with wild card",
			excludeListConfig: map[string][]string{
				"*": {
					memory, nicResourceName,
				},
				"node1": {
					cpu,
					hugepages2Mi,
				},
			},
			nodeName:                  "node2",
			expectedExcludedResources: []string{memory, nicResourceName},
		},
		{
			desc:                      "empty exclude list",
			excludeListConfig:         map[string][]string{},
			nodeName:                  "node1",
			expectedExcludedResources: []string{},
		},
	}

	for _, tt := range tests {
		t.Logf("test %s", tt.desc)
		excludeList := NewExcludeResourceList(tt.excludeListConfig, tt.nodeName)
		for _, res := range tt.expectedExcludedResources {
			if !excludeList.IsExcluded(corev1.ResourceName(res)) {
				t.Errorf("resource: %q expected to be excluded from node: %q", res, tt.nodeName)
			}
		}
	}
}
