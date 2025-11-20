//go:build amd64

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
	"bytes"
	"os"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/cpuid"
)

const (
	// CPUID EAX input values
	LEAF_EXT_FEATURE_FLAGS = 0x07
	LEAF_RDT_MONITORING    = 0x0f
	LEAF_RDT_ALLOCATION    = 0x10

	// CPUID ECX input values
	RDT_MONITORING_SUBLEAF_L3 = 1

	// CPUID bitmasks
	EXT_FEATURE_FLAGS_EBX_RDT_M                                 = 1 << 12
	EXT_FEATURE_FLAGS_EBX_RDT_A                                 = 1 << 15
	RDT_MONITORING_EDX_L3_MONITORING                            = 1 << 1
	RDT_MONITORING_SUBLEAF_L3_EDX_L3_OCCUPANCY_MONITORING       = 1 << 0
	RDT_MONITORING_SUBLEAF_L3_EDX_L3_TOTAL_BANDWIDTH_MONITORING = 1 << 1
	RDT_MONITORING_SUBLEAF_L3_EDX_L3_LOCAL_BANDWIDTH_MONITORING = 1 << 2
	RDT_ALLOCATION_EBX_L3_CACHE_ALLOCATION                      = 1 << 1
	RDT_ALLOCATION_EBX_L2_CACHE_ALLOCATION                      = 1 << 2
	RDT_ALLOCATION_EBX_MEMORY_BANDWIDTH_ALLOCATION              = 1 << 3
)

func discoverRDT() map[string]string {
	attributes := map[string]string{}

	// Read cpuid information
	extFeatures := cpuid.Cpuid(LEAF_EXT_FEATURE_FLAGS, 0)
	rdtMonitoring := cpuid.Cpuid(LEAF_RDT_MONITORING, 0)
	rdtL3Monitoring := cpuid.Cpuid(LEAF_RDT_MONITORING, RDT_MONITORING_SUBLEAF_L3)
	rdtAllocation := cpuid.Cpuid(LEAF_RDT_ALLOCATION, 0)

	// Detect RDT monitoring capabilities
	if extFeatures.EBX&EXT_FEATURE_FLAGS_EBX_RDT_M != 0 {
		if rdtMonitoring.EDX&RDT_MONITORING_EDX_L3_MONITORING != 0 {
			// Monitoring is supported
			attributes["RDTMON"] = "true"

			// Cache Monitoring Technology (L3 occupancy monitoring)
			if rdtL3Monitoring.EDX&RDT_MONITORING_SUBLEAF_L3_EDX_L3_OCCUPANCY_MONITORING != 0 {
				attributes["RDTCMT"] = "true"
			}
			// Memore Bandwidth Monitoring (L3 local&total bandwidth monitoring)
			if rdtL3Monitoring.EDX&RDT_MONITORING_SUBLEAF_L3_EDX_L3_TOTAL_BANDWIDTH_MONITORING != 0 &&
				rdtL3Monitoring.EDX&RDT_MONITORING_SUBLEAF_L3_EDX_L3_LOCAL_BANDWIDTH_MONITORING != 0 {
				attributes["RDTMBM"] = "true"
			}
		}
	}

	// Detect RDT allocation capabilities
	if extFeatures.EBX&EXT_FEATURE_FLAGS_EBX_RDT_A != 0 {
		// L3 Cache Allocation
		if rdtAllocation.EBX&RDT_ALLOCATION_EBX_L3_CACHE_ALLOCATION != 0 {
			attributes["RDTL3CA"] = "true"
			numClosID := getNumClosID("L3")
			if numClosID > -1 {
				attributes["RDTL3CA_NUM_CLOSID"] = strconv.FormatInt(int64(numClosID), 10)
			}
		}
		// L2 Cache Allocation
		if rdtAllocation.EBX&RDT_ALLOCATION_EBX_L2_CACHE_ALLOCATION != 0 {
			attributes["RDTL2CA"] = "true"
		}
		// Memory Bandwidth Allocation
		if rdtAllocation.EBX&RDT_ALLOCATION_EBX_MEMORY_BANDWIDTH_ALLOCATION != 0 {
			attributes["RDTMBA"] = "true"
		}
	}

	return attributes
}

func getNumClosID(level string) int64 {
	resctrlRootDir, err := intelrdt.Root()
	if err != nil {
		klog.V(4).ErrorS(err, "can't find resctrl filesystem")
		return -1
	}

	closidFile := filepath.Join(resctrlRootDir, "info", level, "num_closids")

	if _, err := os.Stat(closidFile); err != nil {
		klog.V(4).ErrorS(err, "failed to stat file", "path", closidFile)
		return -1
	}

	closidsBytes, err := os.ReadFile(filepath.Join(resctrlRootDir, "info", level, "num_closids"))
	if err != nil {
		klog.V(4).ErrorS(err, "failed to read file", "path", closidFile)
		return -1
	}

	numClosIDs, err := strconv.ParseInt(string(bytes.TrimSpace(closidsBytes)), 10, 64)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to parse num_closids", "value", string(bytes.TrimSpace(closidsBytes)))
		return -1
	}

	// subtract 1 for default control group
	return numClosIDs - 1
}
