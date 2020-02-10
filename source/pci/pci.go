/*
Copyright 2018 The Kubernetes Authors.

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
	"io/ioutil"
	"log"
	"path"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

type pciDeviceInfo map[string]string

type NFDConfig struct {
	DeviceClassWhitelist      []string            `json:"deviceClassWhitelist,omitempty"`
	DeviceLabelFieldsForClass map[string][]string `json:"deviceLabelFieldsForClass"`
	DeviceLabelFields         []string            `json:"deviceLabelFields,omitempty"`
}

var Config = NFDConfig{
	DeviceClassWhitelist: []string{"03", "0b40", "12"},
	// Custom fields for selected classes
	DeviceLabelFieldsForClass: map[string][]string{
		"0b40": {"device", "class", "vendor"}, // Class 0b40: Co-processor
	},
	// Default fields for the rest of the classes
	DeviceLabelFields: []string{"class", "vendor"},
}

var devLabelAttrs = []string{} // All device fields mentioned in NFDConfig

// Implement FeatureSource interface
type Source struct{}

func init() {
	// Initialize devLabelAttrs to include all fields in NFDConfig
	seenFields := map[string]bool{}
	for _, fieldList := range Config.DeviceLabelFieldsForClass {
		for _, field := range fieldList {
			seenFields[field] = true
		}
	}
	for _, field := range Config.DeviceLabelFields {
		seenFields[field] = true
	}
	for field, _ := range seenFields {
		devLabelAttrs = append(devLabelAttrs, field)
	}
}

// Return name of the feature source
func (s Source) Name() string { return "pci" }

// Discover features
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	devs, err := detectPci()
	if err != nil {
		return nil, fmt.Errorf("Failed to detect PCI devices: %s", err.Error())
	}

	// Construct a device label format
	for class, classDevs := range devs {
		for _, white := range Config.DeviceClassWhitelist {
			if strings.HasPrefix(class, strings.ToLower(white)) {
				for _, dev := range classDevs {
					devLabel := ""
					var devFields []string
					if _, ok := Config.DeviceLabelFieldsForClass[class]; ok {
						devFields = Config.DeviceLabelFieldsForClass[class]
					} else {
						devFields = Config.DeviceLabelFields
					}
					for i, attr := range devFields {
						devLabel += dev[attr]
						if i < len(devFields)-1 {
							devLabel += "_"
						}
					}
					devLabel += ".present"
					features[devLabel] = true
				}
			}
		}
	}

	return features, nil
}

// Read information of one PCI device
func readDevInfo(devPath string) (pciDeviceInfo, error) {
	info := pciDeviceInfo{}

	for _, attr := range devLabelAttrs {
		data, err := ioutil.ReadFile(path.Join(devPath, attr))
		if err != nil {
			return info, fmt.Errorf("Failed to read device %s: %s", attr, err)
		}
		// Strip whitespace and '0x' prefix
		info[attr] = strings.TrimSpace(strings.TrimPrefix(string(data), "0x"))

		if attr == "class" && len(info[attr]) > 4 {
			// Take four first characters, so that the programming
			// interface identifier gets stripped from the raw class code
			info[attr] = info[attr][0:4]
		}
	}
	return info, nil
}

// List available PCI devices
func detectPci() (map[string][]pciDeviceInfo, error) {
	const basePath = "/sys/bus/pci/devices/"
	devInfo := make(map[string][]pciDeviceInfo)

	devices, err := ioutil.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	// Iterate over devices
	for _, device := range devices {
		info, err := readDevInfo(path.Join(basePath, device.Name()))
		if err != nil {
			log.Print(err)
			continue
		}
		class := info["class"]
		devInfo[class] = append(devInfo[class], info)
	}

	return devInfo, nil
}
