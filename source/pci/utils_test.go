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

package pci

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// writeDevFiles creates dir with the five mandatory PCI attribute files plus
// any extra files given in attrs (file name -> content).
func writeDevFiles(t *testing.T, dir string, attrs map[string]string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	files := map[string]string{
		"class":            "0x020000\n",
		"vendor":           "0x15b3\n",
		"device":           "0x1021\n",
		"subsystem_vendor": "0x15b3\n",
		"subsystem_device": "0x0022\n",
	}
	for name, content := range attrs {
		files[name] = content
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
}

// writeDev creates <root>/bus/pci/devices/<bdf>/ with the mandatory attribute
// files plus any extra files given in attrs.
func writeDev(t *testing.T, root, bdf string, attrs map[string]string) {
	t.Helper()
	writeDevFiles(t, filepath.Join(root, "bus", "pci", "devices", bdf), attrs)
}

// devLink creates the symlink <root>/bus/pci/devices/<bdf>/<name> -> target.
func devLink(t *testing.T, root, bdf, name, target string) {
	t.Helper()
	require.NoError(t, os.Symlink(target, filepath.Join(root, "bus", "pci", "devices", bdf, name)))
}

// pfWithVfs creates a PF with two virtfn links and two VFs, each with a
// physfn link back to the PF. All three devices are on NUMA node 1, so the
// VFs carry both numa_node and physfn and the PF differs only by physfn.
func pfWithVfs(t *testing.T, root, pf, vf0, vf1 string) {
	t.Helper()
	numa1 := map[string]string{"numa_node": "1\n"}
	writeDev(t, root, pf, numa1)
	writeDev(t, root, vf0, numa1)
	writeDev(t, root, vf1, numa1)
	devLink(t, root, pf, "virtfn0", "../"+vf0)
	devLink(t, root, pf, "virtfn1", "../"+vf1)
	devLink(t, root, vf0, "physfn", "../"+pf)
	devLink(t, root, vf1, "physfn", "../"+pf)
}

func TestDetectPciIdentity(t *testing.T) {
	origSysfs := hostpath.SysfsDir
	t.Cleanup(func() { hostpath.SysfsDir = origSysfs })

	tests := []struct {
		name     string
		setup    func(t *testing.T, root string)
		expected []map[string]string
	}{
		{
			name: "nonzero domains same bdf",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.0", map[string]string{"numa_node": "0\n"})
				writeDev(t, root, "0012:01:00.0", map[string]string{"numa_node": "1\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.0", "numa_node": "0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0012:01:00.0", "numa_node": "1"},
			},
		},
		{
			name: "five digit domain verbatim",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "10000:00:01.0", nil)
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "10000:00:01.0"},
			},
		},
		{
			name: "numa node zero with newline",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", map[string]string{"numa_node": "0\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:01:00.0", "numa_node": "0"},
			},
		},
		{
			name: "numa node plus sign normalized",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", map[string]string{"numa_node": "+1\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:01:00.0", "numa_node": "1"},
			},
		},
		{
			name: "numa node unknown values",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", map[string]string{"numa_node": "-1\n"})
				writeDev(t, root, "0000:02:00.0", map[string]string{"numa_node": "\n"})
				writeDev(t, root, "0000:03:00.0", map[string]string{"numa_node": "abc\n"})
				writeDev(t, root, "0000:04:00.0", map[string]string{"numa_node": "-7\n"})
				writeDev(t, root, "0000:05:00.0", map[string]string{"numa_node": " \n"})
				writeDev(t, root, "0000:06:00.0", map[string]string{"numa_node": "99999999999999999999\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:02:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:03:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:04:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:05:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:06:00.0"},
			},
		},
		{
			name: "numa node missing file",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", nil)
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:01:00.0"},
			},
		},
		{
			name: "pf with virtfn links",
			setup: func(t *testing.T, root string) {
				pfWithVfs(t, root, "0002:01:00.0", "0002:01:00.2", "0002:01:00.3")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.0", "numa_node": "1"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.2", "numa_node": "1", "physfn": "0002:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.3", "numa_node": "1", "physfn": "0002:01:00.0"},
			},
		},
		{
			name: "vf pair same pf",
			setup: func(t *testing.T, root string) {
				pfWithVfs(t, root, "0012:01:00.0", "0012:01:00.1", "0012:01:00.2")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0012:01:00.0", "numa_node": "1"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0012:01:00.1", "numa_node": "1", "physfn": "0012:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0012:01:00.2", "numa_node": "1", "physfn": "0012:01:00.0"},
			},
		},
		{
			name: "vf with physfn and unknown numa",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.5", map[string]string{"numa_node": "-1\n"})
				devLink(t, root, "0002:01:00.5", "physfn", "../0002:01:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.5", "physfn": "0002:01:00.0"},
			},
		},
		{
			name: "dangling physfn publishes basename",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.4", nil)
				devLink(t, root, "0002:01:00.4", "physfn", "../0002:01:00.7")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.4", "physfn": "0002:01:00.7"},
			},
		},
		{
			name: "physfn regular file ignored",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.4", map[string]string{"physfn": "0002:01:00.0\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.4"},
			},
		},
		{
			name: "bad device does not erase good",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:05:00.0", nil)
				require.NoError(t, os.Remove(filepath.Join(root, "bus", "pci", "devices", "0000:05:00.0", "vendor")))
				writeDev(t, root, "0000:06:00.0", map[string]string{"numa_node": "0\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0000:06:00.0", "numa_node": "0"},
			},
		},
		{
			name: "address from symlinked entry",
			setup: func(t *testing.T, root string) {
				writeDevFiles(t, filepath.Join(root, "devices", "pci0002:00", "0002:01:00.0"), nil)
				devices := filepath.Join(root, "bus", "pci", "devices")
				require.NoError(t, os.MkdirAll(devices, 0o755))
				require.NoError(t, os.Symlink("../../../devices/pci0002:00/0002:01:00.0", filepath.Join(devices, "0002:01:00.0")))
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "address": "0002:01:00.0"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			hostpath.SysfsDir = hostpath.HostDir(root)

			instances, err := detectPci()
			require.NoError(t, err)

			actual := make([]map[string]string, 0, len(instances))
			for _, inst := range instances {
				actual = append(actual, inst.Attributes)
			}
			sort.SliceStable(actual, func(i, j int) bool {
				return actual[i][AddressAttr] < actual[j][AddressAttr]
			})
			assert.Equal(t, tc.expected, actual)
		})
	}
}
