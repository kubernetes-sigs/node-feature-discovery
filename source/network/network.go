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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "network"

const DeviceFeature = "device"

const sysfsBaseDir = "class/net"

// networkSource implements the FeatureSource and LabelSource interfaces.
type networkSource struct {
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src networkSource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

var (
	// ifaceAttrs is the list of files under /sys/class/net/<iface> that we're trying to read
	ifaceAttrs = []string{"operstate", "speed"}
	// devAttrs is the list of files under /sys/class/net/<iface>/device that we're trying to read
	devAttrs = []string{"sriov_numvfs", "sriov_totalvfs"}
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
		if attrs["operstate"] != "up" {
			continue
		}
		for attr, feature := range map[string]string{
			"sriov_totalvfs": "sriov.capable",
			"sriov_numvfs":   "sriov.configured"} {

			if v, ok := attrs[attr]; ok {
				t, err := strconv.Atoi(v)
				if err != nil {
					klog.Errorf("failed to parse %s of %s: %v", attr, attrs["name"])
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
	s.features = feature.NewDomainFeatures()

	devs, err := detectNetDevices()
	if err != nil {
		return fmt.Errorf("failed to detect network devices: %w", err)
	}
	s.features.Instances[DeviceFeature] = feature.InstanceFeatureSet{Elements: devs}

	utils.KlogDump(3, "discovered network features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface.
func (s *networkSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

func detectNetDevices() ([]feature.InstanceFeature, error) {
	sysfsBasePath := source.SysfsDir.Path(sysfsBaseDir)

	ifaces, err := ioutil.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	// Iterate over devices
	info := make([]feature.InstanceFeature, 0, len(ifaces))
	for _, iface := range ifaces {
		name := iface.Name()
		if _, err := os.Stat(filepath.Join(sysfsBasePath, name, "device")); err == nil {
			info = append(info, readIfaceInfo(filepath.Join(sysfsBasePath, name)))
		} else if klog.V(3).Enabled() {
			klog.Infof("skipping non-device iface %q", name)
		}
	}

	return info, nil
}

func readIfaceInfo(path string) feature.InstanceFeature {
	attrs := map[string]string{"name": filepath.Base(path)}
	for _, attrName := range ifaceAttrs {
		data, err := ioutil.ReadFile(filepath.Join(path, attrName))
		if err != nil {
			if !os.IsNotExist(err) {
				klog.Errorf("failed to read net iface attribute %s: %v", attrName, err)
			}
			continue
		}
		attrs[attrName] = strings.TrimSpace(string(data))
	}

	for _, attrName := range devAttrs {
		data, err := ioutil.ReadFile(filepath.Join(path, "device", attrName))
		if err != nil {
			if !os.IsNotExist(err) {
				klog.Errorf("failed to read net device attribute %s: %v", attrName, err)
			}
			continue
		}
		attrs[attrName] = strings.TrimSpace(string(data))
	}

	return *feature.NewInstanceFeature(attrs)

}

func init() {
	source.Register(&src)
}
