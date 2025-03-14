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
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

var devAttrs = []string{"class", "vendor", "device", "serial"}

// The USB device sysfs files do not have terribly user friendly names, map
// these for consistency with the PCI matcher.
var devAttrFileMap = map[string]string{
	"class":  "bDeviceClass",
	"device": "idProduct",
	"vendor": "idVendor",
	"serial": "serial",
}

func readSingleUsbSysfsAttribute(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read device attribute %s: %w", filepath.Base(path), err)
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
func readUsbDevInfo(devPath string) ([]nfdv1alpha1.InstanceFeature, error) {
	instances := make([]nfdv1alpha1.InstanceFeature, 0)
	attrs := make(map[string]string)

	for _, attr := range devAttrs {
		attrVal, _ := readSingleUsbAttribute(devPath, attr)
		if len(attrVal) > 0 {
			attrs[attr] = attrVal
		}
	}

	// USB devices encode their class information either at the device or the interface level. If the device class
	// is set, return as-is.
	if attrs["class"] != "00" {
		instances = append(instances, *nfdv1alpha1.NewInstanceFeature(attrs))
	} else {
		// Otherwise, if a 00 is presented at the device level, descend to the interface level.
		interfaces, err := filepath.Glob(devPath + "/*/bInterfaceClass")
		if err != nil {
			return nil, err
		}

		// A device may, notably, have multiple interfaces with mixed classes, so we create a unique device for each
		// unique interface class.
		for _, intf := range interfaces {
			// Determine the interface class
			attrVal, err := readSingleUsbSysfsAttribute(intf)
			if err != nil {
				return nil, err
			}

			subdevAttrs := make(map[string]string, len(attrs))
			maps.Copy(subdevAttrs, attrs)
			subdevAttrs["class"] = attrVal

			instances = append(instances, *nfdv1alpha1.NewInstanceFeature(subdevAttrs))
		}
	}

	return instances, nil
}

// detectUsb detects available USB devices and retrieves their device attributes.
func detectUsb() ([]nfdv1alpha1.InstanceFeature, error) {
	// Unlike PCI, the USB sysfs interface includes entries not just for
	// devices. We work around this by globbing anything that includes a
	// valid product ID.
	devPathGlob := hostpath.SysfsDir.Path("bus/usb/devices/*/idProduct")
	devPaths, err := filepath.Glob(devPathGlob)
	if err != nil {
		return nil, err
	}

	// Iterate over devices
	devInfo := make([]nfdv1alpha1.InstanceFeature, 0)
	for _, devPath := range devPaths {
		devs, err := readUsbDevInfo(filepath.Dir(devPath))
		if err != nil {
			klog.ErrorS(err, "failed to read USB device info")
			continue
		}

		devInfo = append(devInfo, devs...)
	}

	return devInfo, nil
}
