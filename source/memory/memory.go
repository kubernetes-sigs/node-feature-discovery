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

package memory

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "memory" }

func GetChoice(path string) (string, error) {
	val, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read '%s': %v", path, err)
	}

	for _, choice := range strings.Fields(string(val)) {
		if len(choice) > 2 && choice[0] == '[' && choice[len(choice)-1] == ']' {
			return choice[1 : len(choice)-1], nil
		}
	}
	return "", nil
}

// Discover returns feature names for memory: numa if more than one memory node is present.
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Find out how many nodes are online
	// Multiple nodes is a sign of NUMA
	bytes, err := ioutil.ReadFile("/sys/devices/system/node/online")
	if err != nil {
		log.Printf("ERROR: can't read /sys/devices/system/node/online: %s", err)
	}

	// File content is expected to be:
	//   "0\n" in one-node case
	//   "0-K\n" in N-node case where K=N-1
	// presence of newline requires TrimSpace
	if strings.TrimSpace(string(bytes)) != "0" {
		// more than one node means NUMA
		features["numa"] = true
	}

	// Check transparent_hugepage
	for _, attrib := range []string{"enabled", "defrag"} {
		choice, err := GetChoice(fmt.Sprintf("/host-sys/kernel/mm/transparent_hugepage/%s", attrib))

		if err != nil {
			log.Printf("ERROR: can't read /host-sys/kernel/mm/transparent_hugepage/%s: %s", attrib, err)
		} else {
			features[fmt.Sprintf("transparent_hugepage.%s", attrib)] = choice
		}

	}

	return features, nil
}
