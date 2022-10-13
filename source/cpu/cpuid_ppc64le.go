/*
Copyright 2019 The Kubernetes Authors.

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
all special features for ppc64le should be defined here; canonical list:
https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/arch/powerpc/include/uapi/asm/cputable.h
*/
const (
	/* AT_HWCAP features */
	PPC_FEATURE_32                     = 0x80000000 /* 32-bit mode. */
	PPC_FEATURE_64                     = 0x40000000 /* 64-bit mode. */
	PPC_FEATURE_601_INSTR              = 0x20000000 /* 601 chip, Old POWER ISA.  */
	PPC_FEATURE_HAS_ALTIVEC            = 0x10000000 /* SIMD/Vector Unit.  */
	PPC_FEATURE_HAS_FPU                = 0x08000000 /* Floating Point Unit.  */
	PPC_FEATURE_HAS_MMU                = 0x04000000 /* Memory Management Unit.  */
	PPC_FEATURE_HAS_4xxMAC             = 0x02000000 /* 4xx Multiply Accumulator.  */
	PPC_FEATURE_UNIFIED_CACHE          = 0x01000000 /* Unified I/D cache.  */
	PPC_FEATURE_HAS_SPE                = 0x00800000 /* Signal Processing ext.  */
	PPC_FEATURE_HAS_EFP_SINGLE         = 0x00400000 /* SPE Float.  */
	PPC_FEATURE_HAS_EFP_DOUBLE         = 0x00200000 /* SPE Double.  */
	PPC_FEATURE_NO_TB                  = 0x00100000 /* 601/403gx have no timebase */
	PPC_FEATURE_POWER4                 = 0x00080000 /* POWER4 ISA 2.00 */
	PPC_FEATURE_POWER5                 = 0x00040000 /* POWER5 ISA 2.02 */
	PPC_FEATURE_POWER5_PLUS            = 0x00020000 /* POWER5+ ISA 2.03 */
	PPC_FEATURE_CELL_BE                = 0x00010000 /* CELL Broadband Engine */
	PPC_FEATURE_BOOKE                  = 0x00008000 /* ISA Category Embedded */
	PPC_FEATURE_SMT                    = 0x00004000 /* Simultaneous Multi-Threading */
	PPC_FEATURE_ICACHE_SNOOP           = 0x00002000
	PPC_FEATURE_ARCH_2_05              = 0x00001000 /* ISA 2.05 */
	PPC_FEATURE_PA6T                   = 0x00000800 /* PA Semi 6T Core */
	PPC_FEATURE_HAS_DFP                = 0x00000400 /* Decimal FP Unit */
	PPC_FEATURE_POWER6_EXT             = 0x00000200 /* P6 + mffgpr/mftgpr */
	PPC_FEATURE_ARCH_2_06              = 0x00000100 /* ISA 2.06 */
	PPC_FEATURE_HAS_VSX                = 0x00000080 /* P7 Vector Extension.  */
	PPC_FEATURE_PSERIES_PERFMON_COMPAT = 0x00000040
	/* Reserved by the kernel.            0x00000004  Do not use.  */
	PPC_FEATURE_TRUE_LE = 0x00000002
	PPC_FEATURE_PPC_LE  = 0x00000001
)

const (
	/* AT_HWCAP2 features */
	PPC_FEATURE2_ARCH_2_07      = 0x80000000 /* ISA 2.07 */
	PPC_FEATURE2_HAS_HTM        = 0x40000000 /* Hardware Transactional Memory */
	PPC_FEATURE2_HAS_DSCR       = 0x20000000 /* Data Stream Control Register */
	PPC_FEATURE2_HAS_EBB        = 0x10000000 /* Event Base Branching */
	PPC_FEATURE2_HAS_ISEL       = 0x08000000 /* Integer Select */
	PPC_FEATURE2_HAS_TAR        = 0x04000000 /* Target Address Register */
	PPC_FEATURE2_HAS_VEC_CRYPTO = 0x02000000 /* Target supports vector instruction.  */
	PPC_FEATURE2_HTM_NOSC       = 0x01000000 /* Kernel aborts transaction when a syscall is made.  */
	PPC_FEATURE2_ARCH_3_00      = 0x00800000 /* ISA 3.0 */
	PPC_FEATURE2_HAS_IEEE128    = 0x00400000 /* VSX IEEE Binary Float 128-bit */
	PPC_FEATURE2_DARN           = 0x00200000 /* darn instruction.  */
	PPC_FEATURE2_SCV            = 0x00100000 /* scv syscall.  */
	PPC_FEATURE2_HTM_NO_SUSPEND = 0x00080000 /* TM without suspended state.  */
	PPC_FEATURE2_ARCH_3_1       = 0x00040000 /* ISA 3.1 */
	PPC_FEATURE2_MMA            = 0x00020000 /* Matrix Multiply Assist */
)

var flagNames_ppc64le = map[uint64]string{
	PPC_FEATURE_32:                     "PPC32",
	PPC_FEATURE_64:                     "PPC64",
	PPC_FEATURE_601_INSTR:              "PPC601",
	PPC_FEATURE_HAS_ALTIVEC:            "ALTIVEC",
	PPC_FEATURE_HAS_FPU:                "FPU",
	PPC_FEATURE_HAS_MMU:                "MMU",
	PPC_FEATURE_HAS_4xxMAC:             "4xxMAC",
	PPC_FEATURE_UNIFIED_CACHE:          "UCACHE",
	PPC_FEATURE_HAS_SPE:                "SPE",
	PPC_FEATURE_HAS_EFP_SINGLE:         "EFPFLOAT",
	PPC_FEATURE_HAS_EFP_DOUBLE:         "EFPDOUBLE",
	PPC_FEATURE_NO_TB:                  "NOTB",
	PPC_FEATURE_POWER4:                 "POWER4",
	PPC_FEATURE_POWER5:                 "POWER5",
	PPC_FEATURE_POWER5_PLUS:            "POWER5+",
	PPC_FEATURE_CELL_BE:                "CELLBE",
	PPC_FEATURE_BOOKE:                  "BOOKE",
	PPC_FEATURE_SMT:                    "SMT",
	PPC_FEATURE_ICACHE_SNOOP:           "IC_SNOOP",
	PPC_FEATURE_ARCH_2_05:              "ARCH_2_05",
	PPC_FEATURE_PA6T:                   "PA6T",
	PPC_FEATURE_HAS_DFP:                "DFP",
	PPC_FEATURE_POWER6_EXT:             "POWER6X",
	PPC_FEATURE_ARCH_2_06:              "ARCH_2_06",
	PPC_FEATURE_HAS_VSX:                "VSX",
	PPC_FEATURE_PSERIES_PERFMON_COMPAT: "ARCHPMU",
	PPC_FEATURE_TRUE_LE:                "TRUE_LE",
	PPC_FEATURE_PPC_LE:                 "PPCLE",
}

var flag2Names_ppc64le = map[uint64]string{
	PPC_FEATURE2_ARCH_2_07:      "ARCH_2_07",
	PPC_FEATURE2_HAS_HTM:        "HTM",
	PPC_FEATURE2_HAS_DSCR:       "DSCR",
	PPC_FEATURE2_HAS_EBB:        "EBB",
	PPC_FEATURE2_HAS_ISEL:       "ISEL",
	PPC_FEATURE2_HAS_TAR:        "TAR",
	PPC_FEATURE2_HAS_VEC_CRYPTO: "VCRYPTO",
	PPC_FEATURE2_HTM_NOSC:       "HTM-NOSC",
	PPC_FEATURE2_ARCH_3_00:      "ARCH_3_00",
	PPC_FEATURE2_HAS_IEEE128:    "IEEE128",
	PPC_FEATURE2_DARN:           "DARN",
	PPC_FEATURE2_SCV:            "SCV",
	PPC_FEATURE2_HTM_NO_SUSPEND: "HTM-NO-SUSPEND",
	PPC_FEATURE2_ARCH_3_1:       "ARCH_3_1",
	PPC_FEATURE2_MMA:            "MMA",
}

func getCpuidFlags() []string {
	r := make([]string, 0, 30)
	hwcap := uint64(C.gethwcap())
	hwcap2 := uint64(C.gethwcap2())
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val, ok := flagNames_ppc64le[key]
		if hwcap&key != 0 && ok {
			r = append(r, val)
		}
	}
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val, ok := flag2Names_ppc64le[key]
		if hwcap2&key != 0 && ok {
			r = append(r, val)
		}
	}
	return r
}
