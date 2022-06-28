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
	"github.com/klauspost/cpuid/v2"
)

func discoverSecurity() map[string]string {
	elems := make(map[string]string)

	if sgxEnabled() {
		elems["sgx.enabled"] = "true"
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
