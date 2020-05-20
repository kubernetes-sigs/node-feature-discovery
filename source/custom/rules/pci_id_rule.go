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

package rules

import (
	"fmt"
	pciutils "sigs.k8s.io/node-feature-discovery/source/internal"
)

// Rule that matches on the following PCI device attributes: <class, vendor, device>
// each device attribute will be a list elements(strings).
// Match operation: OR will be performed per element and AND will be performed per attribute.
// An empty attribute will not be included in the matching process.
type PciIDRuleInput struct {
	Class  []string `json:"class,omitempty"`
	Vendor []string `json:"vendor,omitempty"`
	Device []string `json:"device,omitempty"`
}

type PciIDRule struct {
	PciIDRuleInput
}

// Match PCI devices on provided PCI device attributes
func (r *PciIDRule) Match() (bool, error) {
	devAttr := map[string]bool{}
	for _, attr := range []string{"class", "vendor", "device"} {
		devAttr[attr] = true
	}
	allDevs, err := pciutils.DetectPci(devAttr)
	if err != nil {
		return false, fmt.Errorf("failed to detect PCI devices: %s", err.Error())
	}

	for _, classDevs := range allDevs {
		for _, dev := range classDevs {
			// match rule on a single device
			if r.matchDevOnRule(dev) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *PciIDRule) matchDevOnRule(dev pciutils.PciDeviceInfo) bool {
	if len(r.Class) == 0 && len(r.Vendor) == 0 && len(r.Device) == 0 {
		return false
	}

	if len(r.Class) > 0 && !in(dev["class"], r.Class) {
		return false
	}

	if len(r.Vendor) > 0 && !in(dev["vendor"], r.Vendor) {
		return false
	}

	if len(r.Device) > 0 && !in(dev["device"], r.Device) {
		return false
	}

	return true
}

func in(item string, arr []string) bool {
	for _, val := range arr {
		if val == item {
			return true
		}
	}
	return false
}
