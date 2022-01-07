/*
Copyright 2020-2021 The Kubernetes Authors.

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
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "usb"

const DeviceFeature = "device"

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
		DeviceLabelFields:    defaultDeviceLabelFields(),
	}
}

func defaultDeviceLabelFields() []string { return []string{"class", "vendor", "device"} }

// usbSource implements the LabelSource and ConfigurableSource interfaces.
type usbSource struct {
	config   *Config
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src                           = usbSource{config: newDefaultConfig()}
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)

// Name returns the name of the feature source
func (s *usbSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *usbSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *usbSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *usbSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		klog.Fatalf("invalid config type: %T", conf)
	}
}

// Priority method of the LabelSource interface
func (s *usbSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *usbSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	// Construct a device label format, a sorted list of valid attributes
	deviceLabelFields := []string{}
	configLabelFields := map[string]bool{}
	for _, field := range s.config.DeviceLabelFields {
		configLabelFields[field] = true
	}

	for _, attr := range devAttrs {
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
		klog.Warningf("invalid fields (%s) in deviceLabelFields, ignoring...", strings.Join(keys, ", "))
	}
	if len(deviceLabelFields) == 0 {
		klog.Warningf("no valid fields in deviceLabelFields defined, using the defaults")
		deviceLabelFields = defaultDeviceLabelFields()
	}

	// Iterate over all device classes
	for _, dev := range features.Instances[DeviceFeature].Elements {
		attrs := dev.Attributes
		class := attrs["class"]
		for _, white := range s.config.DeviceClassWhitelist {
			if strings.HasPrefix(string(class), strings.ToLower(white)) {
				devLabel := ""
				for i, attr := range deviceLabelFields {
					devLabel += attrs[attr]
					if i < len(deviceLabelFields)-1 {
						devLabel += "_"
					}
				}
				labels[devLabel+".present"] = true
				break
			}
		}
	}
	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *usbSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	devs, err := detectUsb()
	if err != nil {
		return fmt.Errorf("failed to detect USB devices: %s", err.Error())
	}
	s.features.Instances[DeviceFeature] = feature.NewInstanceFeatures(devs)

	utils.KlogDump(3, "discovered usb features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface
func (s *usbSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

func init() {
	source.Register(&src)
}
