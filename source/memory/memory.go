/*
Copyright 2018-2021 The Kubernetes Authors.

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

package memory

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
const Name = "memory"

const NvFeature = "nv"
const NumaFeature = "numa"

// memorySource implements the FeatureSource and LabelSource interfaces.
type memorySource struct {
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src memorySource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

// Name returns an identifier string for this feature source.
func (s *memorySource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *memorySource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *memorySource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	// NUMA
	if isNuma, ok := features.Values[NumaFeature].Elements["is_numa"]; ok && isNuma == "true" {
		labels["numa"] = true
	}

	// NVDIMM
	if len(features.Instances[NvFeature].Elements) > 0 {
		labels["nv.present"] = true
	}
	for _, dev := range features.Instances[NvFeature].Elements {
		if dev.Attributes["devtype"] == "nd_dax" {
			labels["nv.dax"] = true
			break
		}
	}

	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *memorySource) Discover() error {
	s.features = feature.NewDomainFeatures()

	// Detect NUMA
	if numa, err := detectNuma(); err != nil {
		klog.Errorf("failed to detect NUMA nodes: %v", err)
	} else {
		s.features.Values[NumaFeature] = feature.ValueFeatureSet{Elements: numa}
	}

	// Detect NVDIMM
	if nv, err := detectNv(); err != nil {
		klog.Errorf("failed to detect nvdimm devices: %v", err)
	} else {
		s.features.Instances[NvFeature] = feature.InstanceFeatureSet{Elements: nv}
	}

	utils.KlogDump(3, "discovered memory features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface.
func (s *memorySource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

// detectNuma detects NUMA node information
func detectNuma() (map[string]string, error) {
	sysfsBasePath := source.SysfsDir.Path("bus/node/devices")

	nodes, err := ioutil.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list numa nodes: %w", err)
	}

	return map[string]string{
		"is_numa":    strconv.FormatBool(len(nodes) > 1),
		"node_count": strconv.Itoa(len(nodes)),
	}, nil
}

// detectNv detects NVDIMM devices
func detectNv() ([]feature.InstanceFeature, error) {
	sysfsBasePath := source.SysfsDir.Path("bus/nd/devices")
	info := make([]feature.InstanceFeature, 0)

	devices, err := ioutil.ReadDir(sysfsBasePath)
	if os.IsNotExist(err) {
		klog.V(1).Info("No NVDIMM devices present")
		return info, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to list nvdimm devices: %w", err)
	}

	// Iterate over devices
	for _, device := range devices {
		i := readNdDeviceInfo(filepath.Join(sysfsBasePath, device.Name()))
		info = append(info, i)
	}

	return info, nil
}

// ndDevAttrs is the list of sysfs files (under each nd device) that we're trying to read
var ndDevAttrs = []string{"devtype", "mode"}

func readNdDeviceInfo(path string) feature.InstanceFeature {
	attrs := map[string]string{"name": filepath.Base(path)}
	for _, attrName := range ndDevAttrs {
		data, err := ioutil.ReadFile(filepath.Join(path, attrName))
		if err != nil {
			klog.V(3).Infof("failed to read nd device attribute %s: %w", attrName, err)
			continue
		}
		attrs[attrName] = strings.TrimSpace(string(data))
	}
	return *feature.NewInstanceFeature(attrs)
}

func init() {
	source.Register(&src)
}
