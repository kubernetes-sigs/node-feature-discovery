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

// linkedDev creates the entry <root>/bus/pci/devices/<bdf> as a symlink to
// target and the mandatory attribute files in the directory it points to.
func linkedDev(t *testing.T, root, bdf, target string) {
	t.Helper()
	devices := filepath.Join(root, "bus", "pci", "devices")
	dir := target
	if !filepath.IsAbs(target) {
		dir = filepath.Join(devices, target)
	}
	writeDevFiles(t, dir, nil)
	require.NoError(t, os.MkdirAll(devices, 0o755))
	require.NoError(t, os.Symlink(target, filepath.Join(devices, bdf)))
}

// pfWithVfs creates a PF with two virtfn links and two VFs, each with a
// physfn link back to the PF. All three devices are on NUMA node 1, so the
// VFs carry both numaNode and physfn and the PF differs only by physfn.
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
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.0", "numaNode": "0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.0", "numaNode": "1"},
			},
		},
		{
			name: "five digit domain verbatim",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "10000:00:01.0", nil)
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "10000:00:01.0"},
			},
		},
		{
			name: "numa node zero with newline",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", map[string]string{"numa_node": "0\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0", "numaNode": "0"},
			},
		},
		{
			name: "numa node plus sign normalized",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", map[string]string{"numa_node": "+1\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0", "numaNode": "1"},
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
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:02:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:03:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:04:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:05:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:06:00.0"},
			},
		},
		{
			name: "numa node missing file",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0000:01:00.0", nil)
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0"},
			},
		},
		{
			name: "pf with virtfn links",
			setup: func(t *testing.T, root string) {
				pfWithVfs(t, root, "0002:01:00.0", "0002:01:00.2", "0002:01:00.3")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.0", "numaNode": "1"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.2", "numaNode": "1", "physfn": "0002:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.3", "numaNode": "1", "physfn": "0002:01:00.0"},
			},
		},
		{
			name: "vf pair same pf",
			setup: func(t *testing.T, root string) {
				pfWithVfs(t, root, "0012:01:00.0", "0012:01:00.1", "0012:01:00.2")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.0", "numaNode": "1"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.1", "numaNode": "1", "physfn": "0012:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.2", "numaNode": "1", "physfn": "0012:01:00.0"},
			},
		},
		{
			name: "vf with physfn and unknown numa",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.0", nil)
				writeDev(t, root, "0002:01:00.5", map[string]string{"numa_node": "-1\n"})
				devLink(t, root, "0002:01:00.5", "physfn", "../0002:01:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.5", "physfn": "0002:01:00.0"},
			},
		},
		{
			name: "dangling physfn dropped",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.4", map[string]string{"numa_node": "0\n"})
				devLink(t, root, "0002:01:00.4", "physfn", "../0002:01:00.7")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.4", "numaNode": "0"},
			},
		},
		{
			name: "physfn regular file ignored",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.4", map[string]string{"physfn": "0002:01:00.0\n"})
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.4"},
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
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:06:00.0", "numaNode": "0"},
			},
		},
		{
			name: "pciBusID from symlinked entry",
			setup: func(t *testing.T, root string) {
				writeDevFiles(t, filepath.Join(root, "devices", "pci0002:00", "0002:01:00.0"), nil)
				devices := filepath.Join(root, "bus", "pci", "devices")
				require.NoError(t, os.MkdirAll(devices, 0o755))
				require.NoError(t, os.Symlink("../../../devices/pci0002:00/0002:01:00.0", filepath.Join(devices, "0002:01:00.0")))
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.0", "pcieRoot": "pci0002:00"},
			},
		},
		{
			name: "pcieRoot behind vmd domain",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "10000:01:00.0", "../../../devices/pci0000:00/0000:00:0e.0/pci10000:00/10000:00:02.0/10000:01:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "10000:01:00.0", "pcieRoot": "pci0000:00"},
			},
		},
		{
			name: "pcieRoot behind bridge",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:3b:00.0", "../../../devices/pci0000:3a/0000:3a:00.0/0000:3b:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:3b:00.0", "pcieRoot": "pci0000:3a"},
			},
		},
		{
			name: "pcieRoot absent outside devices pci",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:01:00.0", "../../../devices/platform/soc/0000:01:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0"},
			},
		},
		{
			name: "pcieRoot absent on basename mismatch",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:01:00.0", "../../../devices/pci0000:00/0000:01:00.1")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0"},
			},
		},
		{
			name: "pcieRoot absent for absolute target",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:01:00.0", filepath.Join(root, "devices", "pci0000:00", "0000:01:00.0"))
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0"},
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
				return actual[i]["pciBusID"] < actual[j]["pciBusID"]
			})
			assert.Equal(t, tc.expected, actual)
		})
	}
}

// TestReadPcieRootAbsoluteTarget pins that an absolute link target yields no
// pcieRoot even when, read relative to bus/pci/devices, it would climb back
// into devices/pci. The link dangles, so detectPci cannot reach this case.
func TestReadPcieRootAbsoluteTarget(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "0000:01:00.0")
	require.NoError(t, os.Symlink("/../../../devices/pci0000:00/0000:01:00.0", entry))

	pcieRoot, ok := readPcieRoot(entry)
	assert.False(t, ok)
	assert.Equal(t, "", pcieRoot)
}
