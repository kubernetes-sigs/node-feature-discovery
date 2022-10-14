/*
Copyright 2020 The Kubernetes Authors.

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

/*
#include <sys/auxv.h>

unsigned long gethwcap() {
	return getauxval(AT_HWCAP);
}
unsigned long gethwcap2() {
	return getauxval(AT_HWCAP2);
}
*/
import "C"

/*
all special features for arm should be defined here; canonical list:
https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/arch/arm/include/uapi/asm/hwcap.h
*/
const (
	/* extension instructions */
	CPU_ARM_FEATURE_SWP = 1 << iota
	CPU_ARM_FEATURE_HALF
	CPU_ARM_FEATURE_THUMB
	CPU_ARM_FEATURE_26BIT
	CPU_ARM_FEATURE_FASTMUL
	CPU_ARM_FEATURE_FPA
	CPU_ARM_FEATURE_VFP
	CPU_ARM_FEATURE_EDSP
	CPU_ARM_FEATURE_JAVA
	CPU_ARM_FEATURE_IWMMXT
	CPU_ARM_FEATURE_CRUNCH
	CPU_ARM_FEATURE_THUMBEE
	CPU_ARM_FEATURE_NEON
	CPU_ARM_FEATURE_VFPv3
	CPU_ARM_FEATURE_VFPv3D16
	CPU_ARM_FEATURE_TLS
	CPU_ARM_FEATURE_VFPv4
	CPU_ARM_FEATURE_IDIVA
	CPU_ARM_FEATURE_IDIVT
	CPU_ARM_FEATURE_VFPD32
	CPU_ARM_FEATURE_LPAE
	CPU_ARM_FEATURE_EVTSTRM
)

const (
	CPU_ARM_FEATURE2_AES = 1 << iota
	CPU_ARM_FEATURE2_PMULL
	CPU_ARM_FEATURE2_SHA1
	CPU_ARM_FEATURE2_SHA2
	CPU_ARM_FEATURE2_CRC32
)

var flagNames_arm = map[uint64]string{
	CPU_ARM_FEATURE_SWP:      "SWP",
	CPU_ARM_FEATURE_HALF:     "HALF",
	CPU_ARM_FEATURE_THUMB:    "THUMB",
	CPU_ARM_FEATURE_26BIT:    "26BIT",
	CPU_ARM_FEATURE_FASTMUL:  "FASTMUL",
	CPU_ARM_FEATURE_FPA:      "FPA",
	CPU_ARM_FEATURE_VFP:      "VFP",
	CPU_ARM_FEATURE_EDSP:     "EDSP",
	CPU_ARM_FEATURE_JAVA:     "JAVA",
	CPU_ARM_FEATURE_IWMMXT:   "IWMMXT",
	CPU_ARM_FEATURE_CRUNCH:   "CRUNCH",
	CPU_ARM_FEATURE_THUMBEE:  "THUMBEE",
	CPU_ARM_FEATURE_NEON:     "NEON",
	CPU_ARM_FEATURE_VFPv3:    "VFPv3",
	CPU_ARM_FEATURE_VFPv3D16: "VFPv3D16",
	CPU_ARM_FEATURE_TLS:      "TLS",
	CPU_ARM_FEATURE_VFPv4:    "VFPv4",
	CPU_ARM_FEATURE_IDIVA:    "IDIVA",
	CPU_ARM_FEATURE_IDIVT:    "IDIVT",
	CPU_ARM_FEATURE_VFPD32:   "VFPD32",
	CPU_ARM_FEATURE_LPAE:     "LPAE",
	CPU_ARM_FEATURE_EVTSTRM:  "EVTSTRM",
}

var flag2Names_arm = map[uint64]string{
	CPU_ARM_FEATURE2_AES:   "AES",
	CPU_ARM_FEATURE2_PMULL: "PMULL",
	CPU_ARM_FEATURE2_SHA1:  "SHA1",
	CPU_ARM_FEATURE2_SHA2:  "SHA2",
	CPU_ARM_FEATURE2_CRC32: "CRC32",
}

func getCpuidFlags() []string {
	r := make([]string, 0, 20)
	hwcap := uint64(C.gethwcap())
	hwcap2 := uint64(C.gethwcap2())
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val, ok := flagNames_arm[key]
		if hwcap&key != 0 && ok {
			r = append(r, val)
		}
	}
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val, ok := flag2Names_arm[key]
		if hwcap2&key != 0 && ok {
			r = append(r, val)
		}
	}
	return r
}
