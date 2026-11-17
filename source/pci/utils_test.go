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
// physfn link back to the PF.
func pfWithVfs(t *testing.T, root, pf, vf0, vf1 string) {
	t.Helper()
	writeDev(t, root, pf, nil)
	writeDev(t, root, vf0, nil)
	writeDev(t, root, vf1, nil)
	devLink(t, root, pf, "virtfn0", "../"+vf0)
	devLink(t, root, pf, "virtfn1", "../"+vf1)
	devLink(t, root, vf0, "physfn", "../"+pf)
	devLink(t, root, vf1, "physfn", "../"+pf)
}

// switchPort is the sysfs path, relative to bus/pci/devices, of downstream
// port 0000:3c:08.0 of a switch (upstream port 0000:3b:00.0) behind root
// port 0000:3a:00.0 of root complex pci0000:3a.
const switchPort = "../../../devices/pci0000:3a/0000:3a:00.0/0000:3b:00.0/0000:3c:08.0"

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
				writeDev(t, root, "0002:01:00.0", nil)
				writeDev(t, root, "0012:01:00.0", nil)
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.0"},
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
			name: "pf with virtfn links",
			setup: func(t *testing.T, root string) {
				pfWithVfs(t, root, "0002:01:00.0", "0002:01:00.2", "0002:01:00.3")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.2", "physfn": "0002:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.3", "physfn": "0002:01:00.0"},
			},
		},
		{
			name: "vf pair same pf",
			setup: func(t *testing.T, root string) {
				pfWithVfs(t, root, "0012:01:00.0", "0012:01:00.1", "0012:01:00.2")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.1", "physfn": "0012:01:00.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0012:01:00.2", "physfn": "0012:01:00.0"},
			},
		},
		{
			name: "dangling physfn dropped",
			setup: func(t *testing.T, root string) {
				writeDev(t, root, "0002:01:00.4", nil)
				devLink(t, root, "0002:01:00.4", "physfn", "../0002:01:00.7")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0002:01:00.4"},
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
				writeDev(t, root, "0000:06:00.0", nil)
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:06:00.0"},
			},
		},
		{
			name: "device on root bus has pcieRoot and no pciePath",
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
			name: "pcieRoot and pciePath behind vmd domain",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "10000:01:00.0", "../../../devices/pci0000:00/0000:00:0e.0/pci10000:00/10000:00:02.0/10000:01:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "10000:01:00.0", "pcieRoot": "pci0000:00", "pciePath": "0000:00:0e.0/pci10000:00/10000:00:02.0"},
			},
		},
		{
			name: "pcieRoot and pciePath behind root port",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:3b:00.0", "../../../devices/pci0000:3a/0000:3a:00.0/0000:3b:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:3b:00.0", "pcieRoot": "pci0000:3a", "pciePath": "0000:3a:00.0"},
			},
		},
		{
			name: "pciePath through switch downstream ports",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:3d:00.0", switchPort+"/0000:3d:00.0")
				linkedDev(t, root, "0000:3e:00.0", "../../../devices/pci0000:3a/0000:3a:00.0/0000:3b:00.0/0000:3c:10.0/0000:3e:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:3d:00.0", "pcieRoot": "pci0000:3a", "pciePath": "0000:3a:00.0/0000:3b:00.0/0000:3c:08.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:3e:00.0", "pcieRoot": "pci0000:3a", "pciePath": "0000:3a:00.0/0000:3b:00.0/0000:3c:10.0"},
			},
		},
		{
			name: "vf behind switch carries pciePath and physfn",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:3d:00.0", switchPort+"/0000:3d:00.0")
				linkedDev(t, root, "0000:3d:00.2", switchPort+"/0000:3d:00.2")
				devLink(t, root, "0000:3d:00.2", "physfn", "../0000:3d:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:3d:00.0", "pcieRoot": "pci0000:3a", "pciePath": "0000:3a:00.0/0000:3b:00.0/0000:3c:08.0"},
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:3d:00.2", "pcieRoot": "pci0000:3a", "pciePath": "0000:3a:00.0/0000:3b:00.0/0000:3c:08.0", "physfn": "0000:3d:00.0"},
			},
		},
		{
			name: "pcieRoot and pciePath absent outside devices pci",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:01:00.0", "../../../devices/platform/soc/0000:01:00.0")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:01:00.0"},
			},
		},
		{
			name: "pcieRoot and pciePath absent on basename mismatch",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:02:00.0", "../../../devices/pci0000:00/0000:00:01.0/0000:02:00.1")
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:02:00.0"},
			},
		},
		{
			name: "pcieRoot and pciePath absent for absolute target",
			setup: func(t *testing.T, root string) {
				linkedDev(t, root, "0000:02:00.0", filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0"))
			},
			expected: []map[string]string{
				{"class": "0200", "vendor": "15b3", "device": "1021", "subsystem_vendor": "15b3", "subsystem_device": "0022", "pciBusID": "0000:02:00.0"},
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
				return actual[i][PCIBusIDAttr] < actual[j][PCIBusIDAttr]
			})
			assert.Equal(t, tc.expected, actual)
		})
	}
}

// TestReadPcieTopologyAbsoluteTarget pins that an absolute link target yields
// no topology even when, read relative to bus/pci/devices, it would climb back
// into devices/pci. The link dangles, so detectPci cannot reach this case.
func TestReadPcieTopologyAbsoluteTarget(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "0000:02:00.0")
	require.NoError(t, os.Symlink("/../../../devices/pci0000:00/0000:00:01.0/0000:02:00.0", entry))

	pcieRoot, ports, ok := readPcieTopology(entry)
	assert.False(t, ok)
	assert.Equal(t, "", pcieRoot)
	assert.Empty(t, ports)
}

// TestReadPcieTopologyRootOnlyTarget pins that a link resolving to the root
// complex itself keeps the pcieRoot DRA would derive and yields no ports,
// instead of slicing past the end of the path.
func TestReadPcieTopologyRootOnlyTarget(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "pci0000:00")
	require.NoError(t, os.Symlink("../../../devices/pci0000:00", entry))

	pcieRoot, ports, ok := readPcieTopology(entry)
	assert.True(t, ok)
	assert.Equal(t, "pci0000:00", pcieRoot)
	assert.Empty(t, ports)
}
