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

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

func TestSystemSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)
}

func TestGetDmiIDAttribute(t *testing.T) {
	origSysfsDir := hostpath.SysfsDir
	t.Cleanup(func() { hostpath.SysfsDir = origSysfsDir })

	dmiDir := t.TempDir()
	hostpath.SysfsDir = hostpath.HostDir(dmiDir)

	dmiIDDir := filepath.Join(dmiDir, "devices", "virtual", "dmi", "id")
	err := os.MkdirAll(dmiIDDir, 0755)
	assert.Nil(t, err)

	t.Run("reads attribute value with trimmed whitespace", func(t *testing.T) {
		err := os.WriteFile(filepath.Join(dmiIDDir, "sys_vendor"), []byte("LENOVO\n"), 0644)
		assert.Nil(t, err)

		val, err := getDmiIDAttribute("sys_vendor")
		assert.Nil(t, err)
		assert.Equal(t, "LENOVO", val)
	})

	t.Run("returns error for non-existent attribute", func(t *testing.T) {
		_, err := getDmiIDAttribute("nonexistent")
		assert.NotNil(t, err)
	})
}

func TestDmiIDAttributeNames(t *testing.T) {
	origSysfsDir := hostpath.SysfsDir
	origEtcDir := hostpath.EtcDir
	t.Cleanup(func() {
		hostpath.SysfsDir = origSysfsDir
		hostpath.EtcDir = origEtcDir
	})

	tmpDir := t.TempDir()
	hostpath.SysfsDir = hostpath.HostDir(tmpDir)
	hostpath.EtcDir = hostpath.HostDir(tmpDir)

	dmiIDDir := filepath.Join(tmpDir, "devices", "virtual", "dmi", "id")
	err := os.MkdirAll(dmiIDDir, 0755)
	assert.Nil(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "os-release"), []byte("ID=test\nVERSION_ID=1.0\n"), 0644)
	assert.Nil(t, err)

	expectedAttrs := map[string]string{
		"bios_date":         "01/01/2024",
		"bios_vendor":       "TestBIOSVendor",
		"bios_version":      "1.0.0",
		"board_asset_tag":   "BoardAsset123",
		"board_name":        "TestBoard",
		"board_vendor":      "TestBoardVendor",
		"board_version":     "Rev 1.0",
		"chassis_asset_tag": "ChassisAsset456",
		"chassis_type":      "17",
		"chassis_vendor":    "TestChassisVendor",
		"chassis_version":   "N/A",
		"product_family":    "TestFamily",
		"product_name":      "TestProduct",
		"product_sku":       "SKU-001",
		"product_version":   "2.0",
		"sys_vendor":        "TestVendor",
	}

	for name, value := range expectedAttrs {
		err := os.WriteFile(filepath.Join(dmiIDDir, name), []byte(value+"\n"), 0644)
		assert.Nil(t, err)
	}

	s := &systemSource{}
	err = s.Discover()
	assert.Nil(t, err)

	features := s.GetFeatures()
	dmiFeatures := features.Attributes[DmiIdFeature]
	assert.NotNil(t, dmiFeatures)

	for name, expected := range expectedAttrs {
		actual, exists := dmiFeatures.Elements[name]
		assert.True(t, exists, "DMI attribute %q should exist", name)
		assert.Equal(t, expected, actual, "DMI attribute %q value mismatch", name)
	}
}
