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
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/source"
)

const Name = "memory"

// memorySource implements the LabelSource interface.
type memorySource struct{}

// Singleton source instance
var (
	src memorySource
	_   source.LabelSource = &src
)

// Name returns an identifier string for this feature source.
func (s *memorySource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *memorySource) Priority() int { return 0 }

// Discover returns feature names for memory: numa if more than one memory node is present.
func (s *memorySource) Discover() (source.FeatureLabels, error) {
	features := source.FeatureLabels{}

	// Detect NUMA
	numa, err := isNuma()
	if err != nil {
		klog.Errorf("failed to detect NUMA topology: %s", err)
	} else if numa {
		features["numa"] = true
	}

	// Detect NVDIMM
	nv, err := detectNvdimm()
	if err != nil {
		klog.Errorf("NVDIMM detection failed: %s", err)
	} else {
		for k, v := range nv {
			features["nv."+k] = v
		}
	}

	return features, nil
}

// Detect if the platform has NUMA topology
func isNuma() (bool, error) {
	// Find out how many nodes are online
	// Multiple nodes is a sign of NUMA
	bytes, err := ioutil.ReadFile(source.SysfsDir.Path("devices/system/node/online"))
	if err != nil {
		return false, err
	}

	// File content is expected to be:
	//   "0\n" in one-node case
	//   "0-K\n" in N-node case where K=N-1
	// presence of newline requires TrimSpace
	if strings.TrimSpace(string(bytes)) != "0" {
		// more than one node means NUMA
		return true, nil
	}
	return false, nil
}

// Detect NVDIMM devices and configuration
func detectNvdimm() (map[string]bool, error) {
	features := make(map[string]bool)

	// Check presence of physical devices
	devices, err := ioutil.ReadDir(source.SysfsDir.Path("class/nd"))
	if err == nil {
		if len(devices) > 0 {
			features["present"] = true
		}
	} else if os.IsNotExist(err) {
		return nil, nil
	} else {
		return nil, err
	}

	// Check presence of DAX-configured regions
	devices, err = ioutil.ReadDir(source.SysfsDir.Path("bus/nd/devices"))
	if err == nil {
		for _, d := range devices {
			if strings.HasPrefix(d.Name(), "dax") {
				features["dax"] = true
			}
		}
	} else {
		klog.Warningf("failed to detect NVDIMM configuration: %s", err)
	}

	return features, nil
}

func init() {
	source.Register(&src)
}
