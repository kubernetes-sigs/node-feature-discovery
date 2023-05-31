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
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/cpuid"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

const (
	// LEAF_PROCESSOR_FREQUENCY_INFORMATION is the cpuid leaf to get processor frequency information
	LEAF_PROCESSOR_FREQUENCY_INFORMATION = 0x16
)

func discoverSST() map[string]string {
	features := make(map[string]string)

	if bf, err := discoverSSTBF(); err != nil {
		klog.ErrorS(err, "failed to detect SST-BF")
	} else if bf {
		features["bf.enabled"] = strconv.FormatBool(bf)
	}

	return features
}

func discoverSSTBF() (bool, error) {
	// Get processor's "nominal base frequency" (in MHz) from CPUID
	freqInfo := cpuid.Cpuid(LEAF_PROCESSOR_FREQUENCY_INFORMATION, 0)
	nominalBaseFrequency := int(freqInfo.EAX)

	// Loop over all CPUs in the system
	files, err := os.ReadDir(hostpath.SysfsDir.Path("bus/cpu/devices"))

	if err != nil {
		return false, err
	}
	for _, file := range files {
		// Try to read effective base frequency of each cpu in the system
		filePath := hostpath.SysfsDir.Path("bus/cpu/devices", file.Name(), "cpufreq/base_frequency")
		data, err := os.ReadFile(filePath)
		if os.IsNotExist(err) {
			// Ignore missing file and continue to check other CPUs
			continue
		} else if err != nil {
			return false, err
		}

		effectiveBaseFreq, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return false, fmt.Errorf("non-integer value of %q: %v", filePath, err)
		}

		// Sanity check: Return an error (we don't have enough information to
		// make a decision) if we were able to read effective base frequency,
		// but, CPUID didn't support frequency info
		if nominalBaseFrequency == 0 {
			return false, fmt.Errorf("failed to determine if SST-BF is enabled: nominal base frequency info is missing")
		}

		// If the effective base freq of a CPU is greater than the nominal
		// base freq, we determine that SST-BF has been enabled
		if effectiveBaseFreq/1000 > nominalBaseFrequency {
			return true, nil
		}
	}

	return false, nil
}
