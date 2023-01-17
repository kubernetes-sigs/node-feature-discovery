//go:build amd64
// +build amd64

/*
Copyright 2021 The Kubernetes Authors.

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
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/klauspost/cpuid/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

func discoverSecurity() map[string]string {
	elems := make(map[string]string)

	if sgxEnabled() {
		elems["sgx.enabled"] = "true"
	}

	if tdxEnabled() {
		elems["tdx.enabled"] = "true"

		tdxTotalKeys := getCgroupMiscCapacity("tdx")
		if tdxTotalKeys > -1 {
			elems["tdx.total_keys"] = strconv.FormatInt(int64(tdxTotalKeys), 10)
		}
	}

	if sevParameterEnabled("sev") {
		elems["sev.enabled"] = "true"
	}

	if sevParameterEnabled("sev_es") {
		elems["sev.es.enabled"] = "true"
	}

	if sevParameterEnabled("sev_snp") {
		elems["sev.snp.enabled"] = "true"
	}

	return elems
}

func sgxEnabled() bool {
	var epcSize uint64
	if cpuid.CPU.SGX.Available {
		for _, s := range cpuid.CPU.SGX.EPCSections {
			epcSize += s.EPCSize
		}
	}

	// Set to 'true' based a non-zero sum value of SGX EPC section sizes. The
	// kernel checks for IA32_FEATURE_CONTROL.SGX_ENABLE MSR bit but we can't
	// do that as a normal user. Typically the BIOS, when enabling SGX,
	// allocates "Processor Reserved Memory" for SGX EPC so we rely on > 0
	// size here to set "SGX = enabled".
	if epcSize > 0 {
		return true
	}

	return false
}

func tdxEnabled() bool {
	// If /sys/module/kvm_intel/parameters/tdx is not present, or is present
	// with a value different than "Y\n" assume TDX to be unavailable or
	// disabled.
	protVirtHost := hostpath.SysfsDir.Path("module/kvm_intel/parameters/tdx")
	if content, err := os.ReadFile(protVirtHost); err == nil {
		if string(content) == "Y\n" {
			return true
		}
	}
	return false
}

func sevParameterEnabled(parameter string) bool {
	// SEV-SNP is supported and enabled when the kvm module `sev_snp` parameter is set to `Y`
	// SEV-SNP support infers SEV (-ES) support
	sevKvmParameterPath := hostpath.SysfsDir.Path("module/kvm_amd/parameters/", parameter)
	if _, err := os.Stat(sevKvmParameterPath); err == nil {
		if c, err := os.ReadFile(sevKvmParameterPath); err == nil && len(c) > 0 && (c[0] == '1' || c[0] == 'Y') {
			return true
		}
	}
	return false
}

func getCgroupMiscCapacity(resource string) int64 {
	var totalResources int64 = -1

	miscCgroups := hostpath.SysfsDir.Path("fs/cgroup/misc.capacity")
	f, err := os.Open(miscCgroups)
	if err != nil {
		return totalResources
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return totalResources
		}

		if !strings.HasPrefix(string(line), resource) {
			continue
		}

		s := strings.Split(string(line), " ")
		resources, err := strconv.ParseInt(s[1], 10, 64)
		if err != nil {
			return totalResources
		}

		totalResources = resources
		break
	}

	return totalResources
}
