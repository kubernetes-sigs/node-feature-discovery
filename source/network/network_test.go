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

package network

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestNetworkSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)

}

func TestGetLabelsSriovAggregation(t *testing.T) {
	tests := []struct {
		name     string
		devices  []map[string]string
		expected map[string]interface{}
	}{
		{
			name: "single NIC",
			devices: []map[string]string{
				{
					"name":           "eth0",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "2",
				},
			},
			expected: map[string]interface{}{
				"sriov.capable":    true,
				"sriov.configured": true,
				"sriov.total_vfs":  8,
				"sriov.num_vfs":    2,
			},
		},
		{
			name: "multi NIC aggregation",
			devices: []map[string]string{
				{
					"name":           "eth0",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "2",
				},
				{
					"name":           "eth1",
					"sriov_totalvfs": "16",
					"sriov_numvfs":   "4",
				},
			},
			expected: map[string]interface{}{
				"sriov.capable":    true,
				"sriov.configured": true,
				"sriov.total_vfs":  24,
				"sriov.num_vfs":    6,
			},
		},
		{
			name: "netdevs sharing one parent device are counted once",
			devices: []map[string]string{
				{
					// PF netdev
					"name":           "ens1f0np0",
					"bus_info":       "0000:3b:00.0",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "4",
				},
				{
					// switchdev VF representor, parent is the PF
					"name":           "ens1f0r0",
					"bus_info":       "0000:3b:00.0",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "4",
				},
				{
					// second PF of the same NIC
					"name":           "ens1f1np1",
					"bus_info":       "0000:3b:00.1",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "2",
				},
			},
			expected: map[string]interface{}{
				"sriov.capable":    true,
				"sriov.configured": true,
				"sriov.total_vfs":  16,
				"sriov.num_vfs":    6,
			},
		},
		{
			name: "exclude zero totalvfs (still counts num_vfs)",
			devices: []map[string]string{
				{
					"name":           "eth0",
					"sriov_totalvfs": "0",
					"sriov_numvfs":   "4",
				},
			},
			expected: map[string]interface{}{
				"sriov.configured": true,
				"sriov.num_vfs":    4,
			},
		},
		{
			name: "malformed totalvfs",
			devices: []map[string]string{
				{
					"name":           "eth0",
					"sriov_totalvfs": "invalid",
					"sriov_numvfs":   "2",
				},
			},
			expected: map[string]interface{}{
				"sriov.configured": true,
				"sriov.num_vfs":    2,
			},
		},
		{
			name: "malformed numvfs",
			devices: []map[string]string{
				{
					"name":           "eth0",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "garbage",
				},
			},
			expected: map[string]interface{}{
				"sriov.capable":   true,
				"sriov.total_vfs": 8,
			},
		},
		{
			name: "mixed valid and invalid devices",
			devices: []map[string]string{
				{
					"name":           "eth0",
					"sriov_totalvfs": "8",
					"sriov_numvfs":   "2",
				},
				{
					"name":           "eth1",
					"sriov_totalvfs": "invalid",
					"sriov_numvfs":   "4",
				},
				{
					"name":           "eth2",
					"sriov_totalvfs": "0",
					"sriov_numvfs":   "1",
				},
			},
			expected: map[string]interface{}{
				"sriov.capable":    true,
				"sriov.configured": true,
				"sriov.total_vfs":  8,
				"sriov.num_vfs":    7, // 2 + 4 + 1
			},
		},
		{
			name: "no sriov anywhere",
			devices: []map[string]string{
				{
					"name": "eth0",
				},
			},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &networkSource{
				features: nfdv1alpha1.NewFeatures(),
			}

			var elems []nfdv1alpha1.InstanceFeature
			for _, dev := range tt.devices {
				elems = append(elems, *nfdv1alpha1.NewInstanceFeature(dev))
			}

			s.features.Instances[DeviceFeature] = nfdv1alpha1.InstanceFeatureSet{
				Elements: elems,
			}

			labels, err := s.GetLabels()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate expected labels
			for k, v := range tt.expected {
				got, ok := labels[k]
				if !ok {
					t.Errorf("expected label %q not found", k)
					continue
				}
				if got != v {
					t.Errorf("label %q: expected %v, got %v", k, v, got)
				}
			}

			// Ensure no unexpected labels
			for k := range labels {
				if _, ok := tt.expected[k]; !ok {
					t.Errorf("unexpected label %q present", k)
				}
			}
		})
	}
}

func TestReadBusInfo(t *testing.T) {
	// Mimic sysfs: class/net/<iface>/device is a relative symlink to the
	// parent device, shared by the PF and its switchdev representor.
	root := t.TempDir()
	pciDev := filepath.Join(root, "devices", "pci0000:00", "0000:3b:00.0")
	require.NoError(t, os.MkdirAll(pciDev, 0o755))
	for _, iface := range []string{"ens1f0np0", "ens1f0r0"} {
		ifaceDir := filepath.Join(root, "class", "net", iface)
		require.NoError(t, os.MkdirAll(ifaceDir, 0o755))
		require.NoError(t, os.Symlink("../../../devices/pci0000:00/0000:3b:00.0", filepath.Join(ifaceDir, "device")))
		assert.Equal(t, "0000:3b:00.0", readBusInfo(ifaceDir))
	}

	// No parent device
	noDev := filepath.Join(root, "class", "net", "lo")
	require.NoError(t, os.MkdirAll(noDev, 0o755))
	assert.Empty(t, readBusInfo(noDev))
}
