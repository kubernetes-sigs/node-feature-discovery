/*
Copyright 2020 The Kubernetes Authors.

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

package usb

import (
	"fmt"
	"log"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
	usbutils "sigs.k8s.io/node-feature-discovery/source/internal"
)

type Config struct {
	DeviceClassWhitelist []string `json:"deviceClassWhitelist,omitempty"`
	DeviceLabelFields    []string `json:"deviceLabelFields,omitempty"`
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		// Whitelist specific USB classes: https://www.usb.org/defined-class-codes
		// By default these include classes where different accelerators are typically mapped:
		// Video (0e), Miscellaneous (ef), Application Specific (fe), and Vendor Specific (ff).
		DeviceClassWhitelist: []string{"0e", "ef", "fe", "ff"},
		DeviceLabelFields:    []string{"class", "vendor", "device"},
	}
}

// Implement FeatureSource interface
type Source struct {
	config *Config
}

// Return name of the feature source
func (s Source) Name() string { return "usb" }

// NewConfig method of the FeatureSource interface
func (s *Source) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the FeatureSource interface
func (s *Source) GetConfig() source.Config { return s.config }

// SetConfig method of the FeatureSource interface
func (s *Source) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		log.Printf("PANIC: invalid config type: %T", conf)
	}
}

// Discover features
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Construct a device label format, a sorted list of valid attributes
	deviceLabelFields := []string{}
	configLabelFields := map[string]bool{}
	for _, field := range s.config.DeviceLabelFields {
		configLabelFields[field] = true
	}

	for _, attr := range usbutils.DefaultUsbDevAttrs {
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
		log.Printf("WARNING: invalid fields '%v' in deviceLabelFields, ignoring...", keys)
	}
	if len(deviceLabelFields) == 0 {
		log.Printf("WARNING: no valid fields in deviceLabelFields defined, using the defaults")
		deviceLabelFields = []string{"vendor", "device"}
	}

	// Read configured or default labels. Attributes set to 'true' are considered must-have.
	deviceAttrs := map[string]bool{}
	for _, label := range deviceLabelFields {
		deviceAttrs[label] = true
	}

	devs, err := usbutils.DetectUsb(deviceAttrs)
	if err != nil {
		return nil, fmt.Errorf("Failed to detect USB devices: %s", err.Error())
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
				}
			}
		}
	}
	return features, nil
}
