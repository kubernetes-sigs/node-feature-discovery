/*
Copyright 2017-2021 The Kubernetes Authors.

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

package network

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "network"

const (
	// DeviceFeature exposes physical network devices
	DeviceFeature = "device"
	// VirtualFeature exposes features for network interfaces that are not attached to a physical device
	VirtualFeature = "virtual"
)

const sysfsBaseDir = "class/net"

// networkSource implements the FeatureSource and LabelSource interfaces.
type networkSource struct {
	features *nfdv1alpha1.Features
}

// Singleton source instance
var (
	src networkSource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

var (
	// devIfaceAttrs is the list of files under /sys/class/net/<iface> that we're reading
	devIfaceAttrs = []string{"operstate", "speed", "device/sriov_numvfs", "device/sriov_totalvfs"}

	// virtualIfaceAttrs is the list of files under /sys/class/net/<iface> that we're reading
	virtualIfaceAttrs = []string{"operstate", "speed"}
)

// Name returns an identifier string for this feature source.
func (s *networkSource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *networkSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *networkSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	for _, dev := range features.Instances[DeviceFeature].Elements {
		attrs := dev.Attributes
		for attr, feature := range map[string]string{
			"sriov_totalvfs": "sriov.capable",
			"sriov_numvfs":   "sriov.configured"} {

			if v, ok := attrs[attr]; ok {
				t, err := strconv.Atoi(v)
				if err != nil {
					klog.ErrorS(err, "failed to parse sriov attribute", "attributeName", attr, "deviceName", attrs["name"])
					continue
				}
				if t > 0 {
					labels[feature] = true
				}
			}
		}
	}
	return labels, nil
}

// Discover method of the FeatureSource interface.
func (s *networkSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	devs, virts, err := detectNetDevices()
	if err != nil {
		return fmt.Errorf("failed to detect network devices: %w", err)
	}
	s.features.Instances[DeviceFeature] = nfdv1alpha1.InstanceFeatureSet{Elements: devs}
	s.features.Instances[VirtualFeature] = nfdv1alpha1.InstanceFeatureSet{Elements: virts}

	klog.V(3).InfoS("discovered features", "featureSource", s.Name(), "features", utils.DelayedDumper(s.features))

	return nil
}

// GetFeatures method of the FeatureSource Interface.
func (s *networkSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

func detectNetDevices() ([]nfdv1alpha1.InstanceFeature, []nfdv1alpha1.InstanceFeature, error) {
	sysfsBasePath := hostpath.SysfsDir.Path(sysfsBaseDir)

	ifaces, err := os.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	ifaces = slices.DeleteFunc(ifaces, func(iface os.DirEntry) bool {
		return iface.Name() == "bonding_masters"
	})

	// Iterate over devices
	devIfacesinfo := make([]nfdv1alpha1.InstanceFeature, 0, len(ifaces))
	virtualIfacesinfo := make([]nfdv1alpha1.InstanceFeature, 0, len(ifaces))

	for _, iface := range ifaces {
		name := iface.Name()
		if _, err := os.Stat(filepath.Join(sysfsBasePath, name, "device")); err == nil {
			devIfacesinfo = append(devIfacesinfo, readIfaceInfo(filepath.Join(sysfsBasePath, name), devIfaceAttrs))
		} else {
			virtualIfacesinfo = append(virtualIfacesinfo, readIfaceInfo(filepath.Join(sysfsBasePath, name), virtualIfaceAttrs))
		}
	}

	return devIfacesinfo, virtualIfacesinfo, nil
}

func readIfaceInfo(path string, attrFiles []string) nfdv1alpha1.InstanceFeature {
	attrs := map[string]string{"name": filepath.Base(path)}
	for _, attrFile := range attrFiles {
		data, err := os.ReadFile(filepath.Join(path, attrFile))
		if err != nil {
			if !os.IsNotExist(err) && !errors.Is(err, syscall.EINVAL) {
				klog.ErrorS(err, "failed to read net iface attribute", "attributeName", attrFile)
			}
			continue
		}
		attrName := filepath.Base(attrFile)
		attrs[attrName] = strings.TrimSpace(string(data))
	}

	return *nfdv1alpha1.NewInstanceFeature(attrs)

}

func init() {
	source.Register(&src)
}
