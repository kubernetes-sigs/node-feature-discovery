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

package rules

import (
	"fmt"

	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/cpu"
)

// CpuIDRule implements Rule for the custom source
type CpuIDRule []string

func (cpuids *CpuIDRule) Match() (bool, error) {
	flags, ok := source.GetFeatureSource("cpu").GetFeatures().Keys[cpu.CpuidFeature]
	if !ok {
		return false, fmt.Errorf("cpuid information not available")
	}

	for _, f := range *cpuids {
		if _, ok := flags.Elements[f]; !ok {
			return false, nil
		}
	}
	return true, nil
}
