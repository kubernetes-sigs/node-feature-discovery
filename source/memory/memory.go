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
	"strings"

	"github.com/kubernetes-incubator/node-feature-discovery/source"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "memory" }

// Discover returns feature names for memory: numa if more than one memory node is present.
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Find out how many nodes are online
	// Multiple nodes is a sign of NUMA
	bytes, err := ioutil.ReadFile("/sys/devices/system/node/online")
	if err != nil {
		return nil, fmt.Errorf("can't read /sys/devices/system/node/online: %s", err.Error())
	}
	// File content is expected to be:
	//   "0\n" in one-node case
	//   "0-K\n" in N-node case where K=N-1
	// presence of newline requires TrimSpace
	if strings.TrimSpace(string(bytes)) != "0" {
		// more than one node means NUMA
		features["numa"] = true
	}

	return features, nil
}
