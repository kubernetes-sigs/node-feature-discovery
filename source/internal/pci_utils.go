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

package busutils

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

type PciDeviceInfo map[string]string

var DefaultPciDevAttrs = []string{"class", "vendor", "device", "subsystem_vendor", "subsystem_device"}
var ExtraPciDevAttrs = []string{"sriov_totalvfs"}

// Read a single PCI device attribute
// A PCI attribute in this context, maps to the corresponding sysfs file
func readSinglePciAttribute(devPath string, attrName string) (string, error) {
	data, err := ioutil.ReadFile(path.Join(devPath, attrName))
	if err != nil {
		return "", fmt.Errorf("failed to read device attribute %s: %v", attrName, err)
	}
	// Strip whitespace and '0x' prefix
	attrVal := strings.TrimSpace(strings.TrimPrefix(string(data), "0x"))

	if attrName == "class" && len(attrVal) > 4 {
		// Take four first characters, so that the programming
		// interface identifier gets stripped from the raw class code
		attrVal = attrVal[0:4]
	}
	return attrVal, nil
}

// Read information of one PCI device
func readPciDevInfo(devPath string, deviceAttrSpec map[string]bool) (PciDeviceInfo, error) {
	info := PciDeviceInfo{}

	for attr, must := range deviceAttrSpec {
		attrVal, err := readSinglePciAttribute(devPath, attr)
		if err != nil {
			if must {
				return info, fmt.Errorf("Failed to read device %s: %s", attr, err)
			}
			continue

		}
		info[attr] = attrVal
	}
	return info, nil
}

// List available PCI devices and retrieve device attributes.
// deviceAttrSpec is a map which specifies which attributes to retrieve.
// a false value for a specific attribute marks the attribute as optional.
// a true value for a specific attribute marks the attribute as mandatory.
// "class" attribute is considered mandatory.
// DetectPci() will fail if the retrieval of a mandatory attribute fails.
func DetectPci(deviceAttrSpec map[string]bool) (map[string][]PciDeviceInfo, error) {
	sysfsBasePath := source.SysfsDir.Path("bus/pci/devices")
	devInfo := make(map[string][]PciDeviceInfo)

	devices, err := ioutil.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, err
	}
	// "class" is a mandatory attribute, inject it to spec if needed.
	deviceAttrSpec["class"] = true

	// Iterate over devices
	for _, device := range devices {
		info, err := readPciDevInfo(path.Join(sysfsBasePath, device.Name()), deviceAttrSpec)
		if err != nil {
			log.Print(err)
			continue
		}
		class := info["class"]
		devInfo[class] = append(devInfo[class], info)
	}

	return devInfo, nil
}
