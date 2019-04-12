/*
Copyright 2018 The Kubernetes Authors.

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

package cpu

import (
	"io/ioutil"
	"log"
	"path"

	"sigs.k8s.io/node-feature-discovery/source"
)

const (
	cpuDevicesBaseDir = "/sys/bus/cpu/devices"
)

// Implement FeatureSource interface
type Source struct{}

func (s Source) Name() string { return "cpu" }

func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Check if hyper-threading seems to be enabled
	found, err := haveThreadSiblings()
	if err != nil {
		log.Printf("ERROR: failed to detect hyper-threading: %v", err)
	} else if found {
		features["hardware_multithreading"] = true
	}

	// Check SST-BF
	found, err = discoverSSTBF()
	if err != nil {
		log.Printf("ERROR: failed to detect SST-BF: %v", err)
	} else if found {
		features["power.sst_bf.enabled"] = true
	}

	return features, nil
}

// Check if any (online) CPUs have thread siblings
func haveThreadSiblings() (bool, error) {
	files, err := ioutil.ReadDir(cpuDevicesBaseDir)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		// Try to read siblings from topology
		siblings, err := ioutil.ReadFile(path.Join(cpuDevicesBaseDir, file.Name(), "topology/thread_siblings_list"))
		if err != nil {
			return false, err
		}
		for _, char := range siblings {
			// If list separator found, we determine that there are multiple siblings
			if char == ',' || char == '-' {
				return true, nil
			}
		}
	}
	// No siblings were found
	return false, nil
}
