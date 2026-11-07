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

import (
	"bufio"
	"os"
	"strings"
)

/*
#include <sys/auxv.h>

unsigned long gethwcap() {
	return getauxval(AT_HWCAP);
}
*/
import "C"

/*
all special features for s390x should be defined here; canonical list:
https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/arch/s390/include/asm/elf.h
http://sourceware.org/git/?p=glibc.git;a=blob;f=sysdeps/unix/sysv/linux/s390/bits/hwcap.h;hb=HEAD
*/
const (
	/* AT_HWCAP features */
	HWCAP_S390_ESAN3     = 1
	HWCAP_S390_ZARCH     = 2
	HWCAP_S390_STFLE     = 4
	HWCAP_S390_MSA       = 8
	HWCAP_S390_LDISP     = 16
	HWCAP_S390_EIMM      = 32
	HWCAP_S390_DFP       = 64
	HWCAP_S390_HPAGE     = 128
	HWCAP_S390_ETF3EH    = 256
	HWCAP_S390_HIGH_GPRS = 512
	HWCAP_S390_TE        = 1024
	HWCAP_S390_VX        = 2048
	HWCAP_S390_VXD       = 4096
	HWCAP_S390_VXE       = 8192
	HWCAP_S390_GS        = 16384
	HWCAP_S390_VXRS_EXT2 = 32768
	HWCAP_S390_VXRS_PDE  = 65536
	HWCAP_S390_SORT      = 131072
	HWCAP_S390_DFLT      = 262144
	HWCAP_S390_VXRS_PDE2 = 524288
	HWCAP_S390_NNPA      = 1048576
	HWCAP_S390_PCI_MIO   = 2097152
	HWCAP_S390_SIE       = 4194304
)

var flagNames_s390x = map[uint64]string{
	HWCAP_S390_ESAN3:     "ESAN3",
	HWCAP_S390_ZARCH:     "ZARCH",
	HWCAP_S390_STFLE:     "STFLE",
	HWCAP_S390_MSA:       "MSA",
	HWCAP_S390_LDISP:     "LDISP",
	HWCAP_S390_EIMM:      "EIMM",
	HWCAP_S390_DFP:       "DFP",
	HWCAP_S390_HPAGE:     "EDAT",
	HWCAP_S390_ETF3EH:    "ETF3EH",
	HWCAP_S390_HIGH_GPRS: "HIGHGPRS",
	HWCAP_S390_TE:        "TE",
	HWCAP_S390_VX:        "VX",
	HWCAP_S390_VXD:       "VXD",
	HWCAP_S390_VXE:       "VXE",
	HWCAP_S390_GS:        "GS",
	HWCAP_S390_VXRS_EXT2: "VXE2",
	HWCAP_S390_VXRS_PDE:  "VXP",
	HWCAP_S390_SORT:      "SORT",
	HWCAP_S390_DFLT:      "DFLT",
	HWCAP_S390_VXRS_PDE2: "VXP2",
	HWCAP_S390_NNPA:      "NNPA",
	HWCAP_S390_PCI_MIO:   "PCIMIO",
	HWCAP_S390_SIE:       "SIE",
}

func getCpuidFlags() []string {
	r := make([]string, 0, 20)
	hwcap := uint64(C.gethwcap())
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val, ok := flagNames_s390x[key]
		if hwcap&key != 0 && ok {
			r = append(r, val)
		}
	}
	return r
}

func getCpuidAttributes() map[string]string {
	attrs := make(map[string]string)

	machineType := parseMachineType()
	if machineType != "" {
		attrs["machine_type"] = machineType
		if gen, ok := machineGenerations[machineType]; ok {
			attrs["generation"] = gen
		}
	}

	return attrs
}

// machineGenerations maps s390x machine type codes to generation names.
// Reference: Linux kernel arch/s390/kernel/processor.c setup_elf_platform()
// https://github.com/torvalds/linux/blob/master/arch/s390/kernel/processor.c#L262
var machineGenerations = map[string]string{
	"2817": "z196",
	"2818": "z196",
	"2827": "zEC12",
	"2828": "zEC12",
	"2964": "z13",
	"2965": "z13",
	"3906": "z14",
	"3907": "z14",
	"8561": "z15",
	"8562": "z15",
	"3931": "z16",
	"3932": "z16",
	"9175": "z17",
	"9176": "z17",
}

func parseMachineType() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "processor") {
			continue
		}
		if idx := strings.LastIndex(line, "machine = "); idx != -1 {
			return strings.TrimSpace(line[idx+len("machine = "):])
		}
	}
	return ""
}
