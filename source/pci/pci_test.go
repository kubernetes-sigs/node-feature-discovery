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

package pci

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/source"
)

var packagePath string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	packagePath = filepath.Dir(thisFile)
}

func TestSingletonPciSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)
}

func TestPciSource(t *testing.T) {
	// Specify expected "raw" features. These are always the same for the same
	// mocked sysfs.
	expectedFeatures := map[string]*nfdv1alpha1.Features{
		"rootfs-empty": &nfdv1alpha1.Features{
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances:  map[string]nfdv1alpha1.InstanceFeatureSet{},
		},
		"rootfs-1": &nfdv1alpha1.Features{
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances: map[string]nfdv1alpha1.InstanceFeatureSet{
				"device": nfdv1alpha1.InstanceFeatureSet{
					Elements: []nfdv1alpha1.InstanceFeature{
						{
							Attributes: map[string]string{
								"class":            "0880",
								"device":           "2021",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "ff00",
								"device":           "a1ed",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0106",
								"device":           "a1d2",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "1180",
								"device":           "a1b1",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0780",
								"device":           "a1ba",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0604",
								"device":           "a193",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0c80",
								"device":           "a1a4",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0300",
								"device":           "2000",
								"subsystem_device": "2000",
								"subsystem_vendor": "1a03",
								"vendor":           "1a03",
							},
						},
						{
							Attributes: map[string]string{
								"class":                     "0b40",
								"device":                    "37c8",
								"iommu/intel-iommu/version": "1:0",
								"iommu_group/type":          "identity",
								"sriov_totalvfs":            "16",
								"subsystem_device":          "35cf",
								"subsystem_vendor":          "8086",
								"vendor":                    "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0200",
								"device":           "37d2",
								"sriov_totalvfs":   "32",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
					},
				},
			},
		},
	}

	// Specify test cases
	tests := []struct {
		name           string
		config         *Config
		rootfs         string
		expectErr      bool
		expectedLabels source.FeatureLabels
	}{
		{
			name:   "detect with default config",
			rootfs: "rootfs-1",
			expectedLabels: source.FeatureLabels{
				"0300_1a03.present":       true,
				"0b40_8086.present":       true,
				"0b40_8086.sriov.capable": true,
			},
		},
		{
			name:   "test config with empty DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{},
			},
			expectedLabels: source.FeatureLabels{
				"0c80_8086.present": true,
			},
		},
		{
			name:   "test config with empty DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{},
			},
			expectedLabels: source.FeatureLabels{
				"0c80_8086.present": true,
			},
		},
		{
			name:   "test config with only invalid DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{"foo", "bar"},
			},
			expectedLabels: source.FeatureLabels{
				"0c80_8086.present": true,
			},
		},
		{
			name:   "test config with some invalid DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{"foo", "class"},
			},
			expectedLabels: source.FeatureLabels{
				"0c80.present": true,
			},
		},
		{
			name:           "test empty sysfs",
			rootfs:         "rootfs-empty",
			expectErr:      true,
			expectedLabels: source.FeatureLabels{},
		},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hostpath.SysfsDir = hostpath.HostDir(filepath.Join(packagePath, "testdata", tc.rootfs, "sys"))

			config := tc.config
			if config == nil {
				config = newDefaultConfig()
			}
			testSrc := pciSource{config: config}

			// Discover mock PCI devices
			err := testSrc.Discover()
			if tc.expectErr {
				assert.NotNil(t, err, err)
			} else {
				assert.Nil(t, err, err)
			}

			// Check features
			f := testSrc.GetFeatures()
			assert.Equal(t, expectedFeatures[tc.rootfs], f)

			// Check labels
			l, err := testSrc.GetLabels()
			assert.Nil(t, err, err)
			assert.Equal(t, tc.expectedLabels, l)
		})
	}
}
