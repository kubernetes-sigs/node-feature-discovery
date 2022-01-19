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

package storage

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
)

// Name of this feature source
const Name = "storage"

const BlockFeature = "block"

// storageSource implements the FeatureSource and LabelSource interfaces.
type storageSource struct {
	features *feature.DomainFeatures
}

// Singleton source instance
var (
	src storageSource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

// queueAttrs is the list of files under /sys/block/<dev>/queue that we're trying to read
var queueAttrs = []string{"dax", "rotational", "nr_zones", "zoned"}

// Name returns an identifier string for this feature source.
func (s *storageSource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *storageSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *storageSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	for _, dev := range features.Instances[BlockFeature].Elements {
		if dev.Attributes["rotational"] == "0" {
			labels["nonrotationaldisk"] = true
			break
		}
	}

	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *storageSource) Discover() error {
	s.features = feature.NewDomainFeatures()

	devs, err := detectBlock()
	if err != nil {
		return fmt.Errorf("failed to detect block devices: %w", err)
	}
	s.features.Instances[BlockFeature] = feature.InstanceFeatureSet{Elements: devs}

	utils.KlogDump(3, "discovered storage features:", "  ", s.features)

	return nil
}

// GetFeatures method of the FeatureSource Interface.
func (s *storageSource) GetFeatures() *feature.DomainFeatures {
	if s.features == nil {
		s.features = feature.NewDomainFeatures()
	}
	return s.features
}

func detectBlock() ([]feature.InstanceFeature, error) {
	sysfsBasePath := source.SysfsDir.Path("block")

	blockdevices, err := ioutil.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list block devices: %w", err)
	}

	// Iterate over devices
	info := make([]feature.InstanceFeature, 0, len(blockdevices))
	for _, device := range blockdevices {
		info = append(info, *readBlockDevQueueInfo(filepath.Join(sysfsBasePath, device.Name())))
	}

	return info, nil
}

func readBlockDevQueueInfo(path string) *feature.InstanceFeature {
	attrs := map[string]string{"name": filepath.Base(path)}
	for _, attrName := range queueAttrs {
		data, err := ioutil.ReadFile(filepath.Join(path, "queue", attrName))
		if err != nil {
			klog.V(3).Infof("failed to read block device queue attribute %s: %w", attrName, err)
			continue
		}
		attrs[attrName] = strings.TrimSpace(string(data))
	}
	return feature.NewInstanceFeature(attrs)
}

func init() {
	source.Register(&src)
}
