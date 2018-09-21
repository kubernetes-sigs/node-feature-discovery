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
)

type pciDeviceInfo map[string]string

var logger = log.New(os.Stderr, "", log.LstdFlags)

type NFDConfig struct {
	DeviceClassWhitelist []string `json:"deviceClassWhitelist,omitempty"`
}

var Config = NFDConfig{
	DeviceClassWhitelist: []string{"03", "0b40", "12"},
}

// Implement FeatureSource interface
type Source struct{}

// Return name of the feature source
func (s Source) Name() string { return "pci" }

// Discover features
func (s Source) Discover() ([]string, error) {
	features := map[string]bool{}

	devs, err := detectPci()
	if err != nil {
		return nil, fmt.Errorf("Failed to detect PCI devices: %s", err.Error())
	}

	for class, classDevs := range devs {
		for _, white := range Config.DeviceClassWhitelist {
			if strings.HasPrefix(class, strings.ToLower(white)) {
				for _, dev := range classDevs {
					features[fmt.Sprintf("%s_%s.present", dev["class"], dev["vendor"])] = true
				}
			}
		}
	}

	feature_list := []string{}
	for feature := range features {
		feature_list = append(feature_list, feature)
	}

	return feature_list, nil
}

// Read information of one PCI device
func readDevInfo(devPath string) (pciDeviceInfo, error) {
	info := pciDeviceInfo{}

	attrs := []string{"class", "vendor", "device", "subsystem_vendor", "subsystem_device"}
	for _, attr := range attrs {
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
