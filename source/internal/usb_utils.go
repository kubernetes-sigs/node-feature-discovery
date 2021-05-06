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

package busutils

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type UsbDeviceInfo map[string]string
type UsbClassMap map[string]UsbDeviceInfo

var DefaultUsbDevAttrs = []string{"class", "vendor", "device", "serial"}

// The USB device sysfs files do not have terribly user friendly names, map
// these for consistency with the PCI matcher.
var devAttrFileMap = map[string]string{
	"class":  "bDeviceClass",
	"device": "idProduct",
	"vendor": "idVendor",
	"serial": "serial",
}

func readSingleUsbSysfsAttribute(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read device attribute %s: %v", filepath.Base(path), err)
	}

	attrVal := strings.TrimSpace(string(data))

	return attrVal, nil
}

// Read a single USB device attribute
// A USB attribute in this context, maps to the corresponding sysfs file
func readSingleUsbAttribute(devPath string, attrName string) (string, error) {
	return readSingleUsbSysfsAttribute(path.Join(devPath, devAttrFileMap[attrName]))
}

// Read information of one USB device
func readUsbDevInfo(devPath string, deviceAttrSpec map[string]bool) (UsbClassMap, error) {
	classmap := UsbClassMap{}
	info := UsbDeviceInfo{}

	for attr := range deviceAttrSpec {
		attrVal, _ := readSingleUsbAttribute(devPath, attr)
		if len(attrVal) > 0 {
			info[attr] = attrVal
		}
	}

	// USB devices encode their class information either at the device or the interface level. If the device class
	// is set, return as-is.
	if info["class"] != "00" {
		classmap[info["class"]] = info
	} else {
		// Otherwise, if a 00 is presented at the device level, descend to the interface level.
		interfaces, err := filepath.Glob(devPath + "/*/bInterfaceClass")
		if err != nil {
			return classmap, err
		}

		// A device may, notably, have multiple interfaces with mixed classes, so we create a unique device for each
		// unique interface class.
		for _, intf := range interfaces {
			// Determine the interface class
			attrVal, err := readSingleUsbSysfsAttribute(intf)
			if err != nil {
				return classmap, err
			}

			attr := UsbDeviceInfo{}
			for k, v := range info {
				attr[k] = v
			}

			attr["class"] = attrVal
			classmap[attrVal] = attr
		}
	}

	return classmap, nil
}

// List available USB devices and retrieve device attributes.
// deviceAttrSpec is a map which specifies which attributes to retrieve.
// a false value for a specific attribute marks the attribute as optional.
// a true value for a specific attribute marks the attribute as mandatory.
// "class" attribute is considered mandatory.
// DetectUsb() will fail if the retrieval of a mandatory attribute fails.
func DetectUsb(deviceAttrSpec map[string]bool) (map[string][]UsbDeviceInfo, error) {
	// Unlike PCI, the USB sysfs interface includes entries not just for
	// devices. We work around this by globbing anything that includes a
	// valid product ID.
	const devicePath = "/sys/bus/usb/devices/*/idProduct"
	devInfo := make(map[string][]UsbDeviceInfo)

	devices, err := filepath.Glob(devicePath)
	if err != nil {
		return nil, err
	}

	// "class" is a mandatory attribute, inject it to spec if needed.
	deviceAttrSpec["class"] = true

	// Iterate over devices
	for _, device := range devices {
		devMap, err := readUsbDevInfo(filepath.Dir(device), deviceAttrSpec)
		if err != nil {
			klog.Error(err)
			continue
		}

		for class, info := range devMap {
			devInfo[class] = append(devInfo[class], info)
		}
	}

	return devInfo, nil
}
