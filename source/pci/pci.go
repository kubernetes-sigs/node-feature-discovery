/*
Copyright 2018-2021 The Kubernetes Authors.

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
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/source"
	pciutils "sigs.k8s.io/node-feature-discovery/source/internal"
)

const Name = "pci"

type Config struct {
	DeviceClassWhitelist []string `json:"deviceClassWhitelist,omitempty"`
	DeviceLabelFields    []string `json:"deviceLabelFields,omitempty"`
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		DeviceClassWhitelist: []string{"03", "0b40", "12"},
		DeviceLabelFields:    []string{"class", "vendor"},
	}
}

// pciSource implements the LabelSource and ConfigurableSource interfaces.
type pciSource struct {
	config *Config
}

// Singleton source instance
var (
	src pciSource
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns the name of the feature source
func (s *pciSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *pciSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *pciSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *pciSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		klog.Fatalf("invalid config type: %T", conf)
	}
}

// Priority method of the LabelSource interface
func (s *pciSource) Priority() int { return 0 }

// Discover features
func (s *pciSource) Discover() (source.FeatureLabels, error) {
	features := source.FeatureLabels{}

	// Construct a device label format, a sorted list of valid attributes
	deviceLabelFields := []string{}
	configLabelFields := map[string]bool{}
	for _, field := range s.config.DeviceLabelFields {
		configLabelFields[field] = true
	}

	for _, attr := range pciutils.DefaultPciDevAttrs {
		if _, ok := configLabelFields[attr]; ok {
			deviceLabelFields = append(deviceLabelFields, attr)
			delete(configLabelFields, attr)
		}
	}
	if len(configLabelFields) > 0 {
		keys := []string{}
		for key := range configLabelFields {
			keys = append(keys, key)
		}
		klog.Warningf("invalid fields '%v' in deviceLabelFields, ignoring...", keys)
	}
	if len(deviceLabelFields) == 0 {
		klog.Warningf("no valid fields in deviceLabelFields defined, using the defaults")
		deviceLabelFields = []string{"class", "vendor"}
	}

	// Read extraDevAttrs + configured or default labels. Attributes
	// set to 'true' are considered must-have.
	deviceAttrs := map[string]bool{}
	for _, label := range pciutils.ExtraPciDevAttrs {
		deviceAttrs[label] = false
	}
	for _, label := range deviceLabelFields {
		deviceAttrs[label] = true
	}

	devs, err := pciutils.DetectPci(deviceAttrs)
	if err != nil {
		return nil, fmt.Errorf("failed to detect PCI devices: %s", err.Error())
	}

	// Iterate over all device classes
	for class, classDevs := range devs {
		for _, white := range s.config.DeviceClassWhitelist {
			if strings.HasPrefix(class, strings.ToLower(white)) {
				for _, dev := range classDevs {
					devLabel := ""
					for i, attr := range deviceLabelFields {
						devLabel += dev[attr]
						if i < len(deviceLabelFields)-1 {
							devLabel += "_"
						}
					}
					features[devLabel+".present"] = true

					if _, ok := dev["sriov_totalvfs"]; ok {
						features[devLabel+".sriov.capable"] = true
					}
				}
			}
		}
	}
	return features, nil
}

func init() {
	source.Register(&src)
}
