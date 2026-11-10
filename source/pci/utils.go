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

package pci

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// Attributes of a pci.device instance that identify the device and its
// placement rather than its type. Where Kubernetes DRA defines a standard
// device attribute, the name is its resource.kubernetes.io/ suffix.
const (
	// PCIBusIDAttr is the PCI address (domain:bus:device.function) of the
	// device, as named by its sysfs entry. DRA: resource.kubernetes.io/pciBusID.
	PCIBusIDAttr = "pciBusID"
	// PCIeRootAttr is the PCIe root complex (pci<domain>:<bus>) the device
	// sits under. DRA: resource.kubernetes.io/pcieRoot.
	PCIeRootAttr = "pcieRoot"
	// PCIePathAttr is the '/'-separated list of sysfs path components between
	// the PCIe root complex and the device, root first: usually the root port
	// and any switch ports, and behind Intel VMD also the VMD device and its
	// root bus. It is only present together with pcieRoot, and is omitted for
	// a device directly on the root bus.
	PCIePathAttr = "pciePath"
	// PhysfnAttr is the PCI address of the parent physical function. It is
	// only present on SR-IOV virtual functions.
	PhysfnAttr = "physfn"
)

var mandatoryDevAttrs = []string{"class", "vendor", "device", "subsystem_vendor", "subsystem_device"}
var optionalDevAttrs = []string{"sriov_totalvfs", "iommu_group/type", "iommu/intel-iommu/version"}

// Read a single PCI device attribute
// A PCI attribute in this context, maps to the corresponding sysfs file
func readSinglePciAttribute(devPath string, attrName string) (string, error) {
	data, err := os.ReadFile(filepath.Join(devPath, attrName))
	if err != nil {
		return "", fmt.Errorf("failed to read device attribute %s: %w", attrName, err)
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
func readPciDevInfo(devPath string) (*nfdv1alpha1.InstanceFeature, error) {
	attrs := make(map[string]string)
	for _, attr := range mandatoryDevAttrs {
		attrVal, err := readSinglePciAttribute(devPath, attr)
		if err != nil {
			return nil, fmt.Errorf("failed to read device %s: %w", attr, err)
		}
		attrs[attr] = attrVal
	}
	for _, attr := range optionalDevAttrs {
		attrVal, err := readSinglePciAttribute(devPath, attr)
		if err == nil {
			attrs[attr] = attrVal
		}
	}
	attrs[PCIBusIDAttr] = filepath.Base(devPath)
	if pcieRoot, ports, ok := readPcieTopology(devPath); ok {
		attrs[PCIeRootAttr] = pcieRoot
		if len(ports) > 0 {
			attrs[PCIePathAttr] = strings.Join(ports, "/")
		}
	}
	if physfn, ok := readPhysfn(devPath); ok {
		attrs[PhysfnAttr] = physfn
	}
	return nfdv1alpha1.NewInstanceFeature(attrs), nil
}

// readPcieTopology returns the PCIe root complex of the device and the ports
// between that root complex and the device, root first. devPath is the
// <sysfs>/bus/pci/devices/<name> entry; its relative link target is resolved
// lexically from bus/pci/devices, as DRA resolvePCIeRoot does. The root is the
// first component after devices/, and the ports are the components between
// the root and <name>, so a device directly on the root bus has none. It
// yields ok == false when the entry is not a link, the target is absolute, or
// the target is not a devices/pci... path ending in <name>.
func readPcieTopology(devPath string) (string, []string, bool) {
	target, err := os.Readlink(devPath)
	if err != nil {
		return "", nil, false
	}
	if filepath.IsAbs(target) {
		return "", nil, false
	}
	resolved := filepath.Join("bus", "pci", "devices", target)
	if !strings.HasPrefix(resolved, filepath.Join("devices", "pci")) {
		return "", nil, false
	}
	if filepath.Base(resolved) != filepath.Base(devPath) {
		return "", nil, false
	}
	parts := strings.Split(resolved, string(filepath.Separator))
	var ports []string
	if len(parts) > 3 {
		ports = parts[2 : len(parts)-1]
	}
	return parts[1], ports, true
}

// readPhysfn returns the PCI address of the parent physical function of an
// SR-IOV virtual function, taken from the target of its physfn symlink. It
// yields ok == false when the device has no physfn link, or when the link
// does not resolve, so that an address naming no device is never published.
func readPhysfn(devPath string) (string, bool) {
	link := filepath.Join(devPath, "physfn")
	target, err := os.Readlink(link)
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(link); err != nil {
		return "", false
	}
	return filepath.Base(target), true
}

// detectPci detects available PCI devices and retrieves their device attributes.
// An error is returned if reading any of the mandatory attributes fails.
func detectPci() ([]nfdv1alpha1.InstanceFeature, error) {
	sysfsBasePath := hostpath.SysfsDir.Path("bus/pci/devices")

	devices, err := os.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, err
	}

	// Iterate over devices
	devInfo := make([]nfdv1alpha1.InstanceFeature, 0, len(devices))
	for _, device := range devices {
		info, err := readPciDevInfo(filepath.Join(sysfsBasePath, device.Name()))
		if err != nil {
			klog.ErrorS(err, "failed to read PCI device info")
			continue
		}
		devInfo = append(devInfo, *info)
	}

	return devInfo, nil
}
