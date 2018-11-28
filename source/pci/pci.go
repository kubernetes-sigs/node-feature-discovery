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
	"os"
	"path"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

type pciDeviceInfo map[string]string

var logger = log.New(os.Stderr, "", log.LstdFlags)

type NFDConfig struct {
	DeviceClassWhitelist []string `json:"deviceClassWhitelist,omitempty"`
	DeviceLabelFields    []string `json:"deviceLabelFields,omitempty"`
}

var Config = NFDConfig{
	DeviceClassWhitelist: []string{"03", "0b40", "12"},
	DeviceLabelFields:    []string{"class", "vendor"},
}

var devLabelAttrs = []string{"class", "vendor", "device", "subsystem_vendor", "subsystem_device"}

// Implement FeatureSource interface
type Source struct{}

// Return name of the feature source
func (s Source) Name() string { return "pci" }

// Discover features
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	devs, err := detectPci()
	if err != nil {
		return nil, fmt.Errorf("Failed to detect PCI devices: %s", err.Error())
	}

	// Construct a device label format, a sorted list of valid attributes
	deviceLabelFields := []string{}
	configLabelFields := map[string]bool{}
	for _, field := range Config.DeviceLabelFields {
		configLabelFields[field] = true
	}

	for _, attr := range devLabelAttrs {
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
		deviceLabelFields = []string{"class", "vendor"}
	}

	// Iterate over all device classes
	for class, classDevs := range devs {
		for _, white := range Config.DeviceClassWhitelist {
			if strings.HasPrefix(class, strings.ToLower(white)) {
				for _, dev := range classDevs {
					devLabel := ""
					for i, attr := range deviceLabelFields {
						devLabel += dev[attr]
						if i < len(deviceLabelFields)-1 {
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
