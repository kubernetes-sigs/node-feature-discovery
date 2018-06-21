/*
Copyright 2017 The Kubernetes Authors.

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

package pstate

import (
	"fmt"
	"io/ioutil"
	"runtime"

	"github.com/kubernetes-incubator/node-feature-discovery/source"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "pstate" }

// Discover returns feature names for p-state related features such as turbo boost.
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// On Arm platform, the frequency boost mechanism is software-based.
	// So skip pstate detection on Arm.
	switch runtime.GOARCH {
	case "arm64":
		return features, nil
	}

	// Only looking for turbo boost for now...
	bytes, err := ioutil.ReadFile("/sys/devices/system/cpu/intel_pstate/no_turbo")
	if err != nil {
		return nil, fmt.Errorf("can't detect whether turbo boost is enabled: %s", err.Error())
	}
	if bytes[0] == byte('0') {
		// Turbo boost is enabled.
		features["turbo"] = true
	}

	return features, nil
}
