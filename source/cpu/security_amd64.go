//go:build amd64

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

	// Set to 'true' based a non-zero sum value of SGX EPC section sizes. The
	// kernel checks for IA32_FEATURE_CONTROL.SGX_ENABLE MSR bit but we can't
	// do that as a normal user. Typically the BIOS, when enabling SGX,
	// allocates "Processor Reserved Memory" for SGX EPC so we rely on > 0
	// size here to set "SGX = enabled".
	if epcSize := sgxEnabled(); epcSize > 0 {
		elems["sgx.enabled"] = "true"
		elems["sgx.epc"] = strconv.FormatUint(uint64(epcSize), 10)
	}

	if tdxEnabled() {
		elems["tdx.enabled"] = "true"

		tdxTotalKeys := getCgroupMiscCapacity("tdx")
		if tdxTotalKeys > -1 {
			elems["tdx.total_keys"] = strconv.FormatInt(int64(tdxTotalKeys), 10)
		}
	}

	if tdxProtected() {
		elems["tdx.protected"] = "true"
	}

	if sevParameterEnabled("sev") {
		elems["sev.enabled"] = "true"

		sevAddressSpaceIdentifiers := getCgroupMiscCapacity("sev")
		if sevAddressSpaceIdentifiers > -1 {
			elems["sev.asids"] = strconv.FormatInt(int64(sevAddressSpaceIdentifiers), 10)
		}
	}

	if sevParameterEnabled("sev_es") {
		elems["sev.es.enabled"] = "true"

		sevEncryptedStateIDs := getCgroupMiscCapacity("sev_es")
		if sevEncryptedStateIDs > -1 {
			elems["sev.encrypted_state_ids"] = strconv.FormatInt(int64(sevEncryptedStateIDs), 10)
		}
	}

	if sevParameterEnabled("sev_snp") {
		elems["sev.snp.enabled"] = "true"
	}

	return elems
}

func sgxEnabled() uint64 {
	var epcSize uint64
	if cpuid.CPU.SGX.Available {
		for _, s := range cpuid.CPU.SGX.EPCSections {
			epcSize += s.EPCSize
		}
	}

	return epcSize
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

func tdxProtected() bool {
	return cpuid.CPU.Has(cpuid.TDX_GUEST)
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

func retrieveCgroupMiscCapacityValue(miscCgroupPath *os.File, resource string) int64 {
	var totalResources int64 = -1

	r := bufio.NewReader(miscCgroupPath)
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

func getCgroupMiscCapacity(resource string) int64 {
	miscCgroupsPaths := []string{"fs/cgroup/misc.capacity", "fs/cgroup/misc/misc.capacity"}
	for _, miscCgroupsPath := range miscCgroupsPaths {
		miscCgroups := hostpath.SysfsDir.Path(miscCgroupsPath)
		f, err := os.Open(miscCgroups)
		if err == nil {
			defer f.Close() // nolint: errcheck

			return retrieveCgroupMiscCapacityValue(f, resource)
		}
	}

	return -1
}
