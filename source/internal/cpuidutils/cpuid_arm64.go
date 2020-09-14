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

package cpuidutils

/*
#include <sys/auxv.h>
#define HWCAP_CPUID	(1 << 11)

unsigned long gethwcap() {
	return getauxval(AT_HWCAP);
}
*/
import "C"

/* all special features for arm64 should be defined here */
const (
	/* extension instructions */
	CPU_ARM64_FEATURE_FP = 1 << iota
	CPU_ARM64_FEATURE_ASIMD
	CPU_ARM64_FEATURE_EVTSTRM
	CPU_ARM64_FEATURE_AES
	CPU_ARM64_FEATURE_PMULL
	CPU_ARM64_FEATURE_SHA1
	CPU_ARM64_FEATURE_SHA2
	CPU_ARM64_FEATURE_CRC32
	CPU_ARM64_FEATURE_ATOMICS
	CPU_ARM64_FEATURE_FPHP
	CPU_ARM64_FEATURE_ASIMDHP
	CPU_ARM64_FEATURE_CPUID
	CPU_ARM64_FEATURE_ASIMDRDM
	CPU_ARM64_FEATURE_JSCVT
	CPU_ARM64_FEATURE_FCMA
	CPU_ARM64_FEATURE_LRCPC
	CPU_ARM64_FEATURE_DCPOP
	CPU_ARM64_FEATURE_SHA3
	CPU_ARM64_FEATURE_SM3
	CPU_ARM64_FEATURE_SM4
	CPU_ARM64_FEATURE_ASIMDDP
	CPU_ARM64_FEATURE_SHA512
	CPU_ARM64_FEATURE_SVE
)

var flagNames_arm64 = map[uint64]string{
	CPU_ARM64_FEATURE_FP:       "FP",
	CPU_ARM64_FEATURE_ASIMD:    "ASIMD",
	CPU_ARM64_FEATURE_EVTSTRM:  "EVTSTRM",
	CPU_ARM64_FEATURE_AES:      "AES",
	CPU_ARM64_FEATURE_PMULL:    "PMULL",
	CPU_ARM64_FEATURE_SHA1:     "SHA1",
	CPU_ARM64_FEATURE_SHA2:     "SHA2",
	CPU_ARM64_FEATURE_CRC32:    "CRC32",
	CPU_ARM64_FEATURE_ATOMICS:  "ATOMICS",
	CPU_ARM64_FEATURE_FPHP:     "FPHP",
	CPU_ARM64_FEATURE_ASIMDHP:  "ASIMDHP",
	CPU_ARM64_FEATURE_CPUID:    "CPUID",
	CPU_ARM64_FEATURE_ASIMDRDM: "ASIMDRDM",
	CPU_ARM64_FEATURE_JSCVT:    "JSCVT",
	CPU_ARM64_FEATURE_FCMA:     "FCMA",
	CPU_ARM64_FEATURE_LRCPC:    "LRCPC",
	CPU_ARM64_FEATURE_DCPOP:    "DCPOP",
	CPU_ARM64_FEATURE_SHA3:     "SHA3",
	CPU_ARM64_FEATURE_SM3:      "SM3",
	CPU_ARM64_FEATURE_SM4:      "SM4",
	CPU_ARM64_FEATURE_ASIMDDP:  "ASIMDDP",
	CPU_ARM64_FEATURE_SHA512:   "SHA512",
	CPU_ARM64_FEATURE_SVE:      "SVE",
}

func GetCpuidFlags() []string {
	r := make([]string, 0, 20)
	hwcap := uint64(C.gethwcap())
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val := flagNames_arm64[key]
		if hwcap&key != 0 {
			r = append(r, val)
		}
	}
	return r
}
