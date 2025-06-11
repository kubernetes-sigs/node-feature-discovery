---
title: "Feature labels"
parent: "Usage"
layout: default
nav_order: 1
---

# Feature labels
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

Features are advertised as labels in the Kubernetes Node object.

## Built-in labels

Label creation in nfd-worker is performed by a set of separate modules called
label sources. The
[`core.labelSources`](../reference/worker-configuration-reference.md#corelabelsources)
configuration option (or
[`-label-sources`](../reference/worker-commandline-reference.md#-label-sources)
flag) of nfd-worker controls which sources to enable for label generation.

All built-in labels use the `feature.node.kubernetes.io` label namespace and
have the following format.

```plaintext
feature.node.kubernetes.io/<feature> = <value>
```

> **NOTE:** Consecutive runs of nfd-worker will update the labels on a given
> node. If features are not discovered on a consecutive run, the corresponding
> label will be removed. This includes any restrictions placed on the
> consecutive run, such as restricting discovered features with the
> [`-label-whitelist`](../reference/master-commandline-reference.md#-label-whitelist)
> flag of nfd-master or
> [`core.labelWhiteList`](../reference/worker-configuration-reference.md#corelabelwhitelist)
> option of nfd-worker.

### CPU

| Feature name                        | Value  | Description                                                                 |
| ----------------------------------- | ------ | --------------------------------------------------------------------------- |
| **`cpu-cpuid.<cpuid-flag>`**        | true   | CPU capability is supported. **NOTE:** the capability might be supported but not enabled. |
| **`cpu-cpuid.<cpuid-attribute>`**   | string | CPU attribute value |
| **`cpu-hardware_multithreading`**   | true   | Hardware multithreading, such as Intel HTT, enabled (number of logical CPUs is greater than physical CPUs) |
| **`cpu-coprocessor.nx_gzip`**       | true   | Nest Accelerator for GZIP is supported(Power). |
| **`cpu-power.sst_bf.enabled`**      | true   | Intel SST-BF ([Intel Speed Select Technology][intel-sst] - Base frequency) enabled |
| **`cpu-pstate.status`**             | string | The status of the [Intel pstate][intel-pstate] driver when in use and enabled, either 'active' or 'passive'. |
| **`cpu-pstate.turbo`**              | bool   | Set to 'true' if turbo frequencies are enabled in Intel pstate driver, set to 'false' if they have been disabled. |
| **`cpu-pstate.scaling_governor`**   | string | The value of the Intel pstate scaling_governor when in use, either 'powersave' or 'performance'. |
| **`cpu-cstate.enabled`**            | bool   | Set to 'true' if cstates are set in the intel_idle driver, otherwise set to 'false'. Unset if intel_idle cpuidle driver is not active. |
| **`cpu-security.sgx.enabled`**      | true   | Set to 'true' if Intel SGX is enabled in BIOS (based on a non-zero sum value of SGX EPC section sizes). |
| **`cpu-security.se.enabled`**       | true   | Set to 'true' if IBM Secure Execution for Linux (IBM Z & LinuxONE) is available and enabled (requires `/sys/firmware/uv/prot_virt_host` facility) |
| **`cpu-security.tdx.enabled`**      | true   | Set to 'true' if Intel TDX is available on the host and has been enabled (requires `/sys/module/kvm_intel/parameters/tdx`). |
| **`cpu-security.tdx.protected`**    | true   | Set to 'true' if Intel TDX was used to start the guest node, based on the existence of the "TDX_GUEST" information as part of cpuid features. |
| **`cpu-security.sev.enabled`**      | true   | Set to 'true' if ADM SEV is available on the host and has been enabled (requires `/sys/module/kvm_amd/parameters/sev`). |
| **`cpu-security.sev.es.enabled`**   | true   | Set to 'true' if ADM SEV-ES is available on the host and has been enabled (requires `/sys/module/kvm_amd/parameters/sev_es`). |
| **`cpu-security.sev.snp.enabled`**  | true   | Set to 'true' if ADM SEV-SNP is available on the host and has been enabled (requires `/sys/module/kvm_amd/parameters/sev_snp`). |
| **`cpu-model.vendor_id`**           | string | Comparable CPU vendor ID. |
| **`cpu-model.family`**              | int    | CPU family. |
| **`cpu-model.id`**                  | int    | CPU model number. |

The CPU label source is configurable, see
[worker configuration](nfd-worker.md#worker-configuration) and
[`sources.cpu`](../reference/worker-configuration-reference.md#sourcescpu)
configuration options for details.

#### X86 CPUID flags (partial list)

| Flag               | Description                                             |
| ------------------ | ------------------------------------------------------- |
| ADX                | Multi-Precision Add-Carry Instruction Extensions (ADX) |
| AESNI              | Advanced Encryption Standard (AES) New Instructions (AES-NI) |
| APX_F              | Intel Advanced Performance Extensions (APX) |
| AVX10              | Intel Advanced Vector Extensions 10 (AVX10) |
| AVX10_256, AVX10_512 | Intel AVX10 256-bit and 512-bit vector support |
| AVX                | Advanced Vector Extensions (AVX) |
| AVX2               | Advanced Vector Extensions 2 (AVX2) |
| AVXIFMA            | AVX-IFMA instructions |
| AVXVNNI            | AVX (VEX encoded) VNNI neural network instructions |
| AMXBF16            | Advanced Matrix Extension, tile multiplication operations on BFLOAT16 numbers |
| AMXCOMPLEX         | Advanced Matrix Extension, tile computational operations on complex numbers |
| AMXINT8            | Advanced Matrix Extension, tile multiplication operations on 8-bit integers |
| AMXFP16            | Advanced Matrix Extension, tile multiplication operations on FP16 numbers |
| AMXFP8             | Advanced Matrix Extension, tile multiplication operations on FP8 numbers |
| AMXTILE            | Advanced Matrix Extension, base tile architecture support |
| AMXTF32            | Advanced Matrix Extension, matrix multiplication of TF32 tiles into packed single precision tile |
| AVX512BF16         | AVX-512 BFLOAT16 instructions |
| AVX512BITALG       | AVX-512 bit Algorithms |
| AVX512BW           | AVX-512 byte and word Instructions |
| AVX512CD           | AVX-512 conflict detection instructions |
| AVX512DQ           | AVX-512 doubleword and quadword instructions |
| AVX512ER           | AVX-512 exponential and reciprocal instructions |
| AVX512F            | AVX-512 foundation |
| AVX512FP16         | AVX-512 FP16 instructions |
| AVX512IFMA         | AVX-512 integer fused multiply-add instructions |
| AVX512PF           | AVX-512 prefetch instructions |
| AVX512VBMI         | AVX-512 vector bit manipulation instructions |
| AVX512VBMI2        | AVX-512 vector bit manipulation instructions, version 2 |
| AVX512VL           | AVX-512 vector length extensions |
| AVX512VNNI         | AVX-512 vector neural network instructions |
| AVX512VP2INTERSECT | AVX-512 intersect for D/Q |
| AVX512VPOPCNTDQ    | AVX-512 vector population count doubleword and quadword |
| AVXNECONVERT       | AVX-NE-CONVERT instructions |
| AVXVNNIINT8        | AVX-VNNI-INT8 instructions |
| AVXVNNIINT16       | AVX-VNNI-INT16 instructions |
| CMPCCXADD          | CMPCCXADD instructions |
| ENQCMD             | Enqueue Command |
| GFNI               | Galois Field New Instructions |
| HYPERVISOR         | Running under hypervisor |
| MSRLIST            | Read/Write List of Model Specific Registers |
| PREFETCHI          | PREFETCHIT0/1 instructions |
| VAES               | AVX-512 vector AES instructions |
| VPCLMULQDQ         | Carry-less multiplication quadword |
| WRMSRNS            | Non-Serializing Write to Model Specific Register |

By default, the following CPUID flags have been blacklisted: AVX10 (use
AVX10_VERSION instead), BMI1, BMI2, CLMUL, CMOV, CX16, ERMS, F16C, HTT, LZCNT,
MMX, MMXEXT, NX, POPCNT, RDRAND, RDSEED, RDTSCP, SGX, SSE, SSE2, SSE3, SSE4,
SSE42, SSSE3 and TDX_GUEST. See
[`sources.cpu`](../reference/worker-configuration-reference.md#sourcescpu)
configuration options to change the behavior.

See the full list in [github.com/klauspost/cpuid][klauspost-cpuid].

#### X86 CPUID attributes

| Attribute          | Description                                             |
| ------------------ | ------------------------------------------------------- |
| AVX10_VERSION      | AVX10 vector ISA version (if supported)                 |

#### Arm CPUID flags (partial list)

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| IDIVA     | Integer divide instructions available in ARM mode                 |
| IDIVT     | Integer divide instructions available in Thumb mode               |
| THUMB     | Thumb instructions                                                |
| FASTMUL   | Fast multiplication                                               |
| VFP       | Vector floating point instruction extension (VFP)                 |
| VFPv3     | Vector floating point extension v3                                 |
| VFPv4     | Vector floating point extension v4                                 |
| VFPD32    | VFP with 32 D-registers                                           |
| HALF      | Half-word loads and stores                                        |
| EDSP      | DSP extensions                                                    |
| NEON      | NEON SIMD instructions                                            |
| LPAE      | Large Physical Address Extensions                                  |

#### Arm64 CPUID flags (partial list)

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| AES       | Announcing the Advanced Encryption Standard                       |
| EVSTRM    | Event Stream Frequency Features                                   |
| FPHP      | Half Precision(16bit) Floating Point Data Processing Instructions |
| ASIMDHP   | Half Precision(16bit) Asimd Data Processing Instructions          |
| ATOMICS   | Atomic Instructions to the A64                                    |
| ASIMRDM   | Support for Rounding Double Multiply Add/Subtract                 |
| PMULL     | Optional Cryptographic and CRC32 Instructions                     |
| JSCVT     | Perform Conversion to Match Javascript                            |
| DCPOP     | Persistent Memory Support                                         |

### Kernel

| Feature                      | Value  | Description                                               |
| ----------------------------| ------ | --------------------------------------------------------- |
| **`kernel-config.<option>`** | true   | Kernel config option is enabled (set 'y' or 'm'). Default options are `NO_HZ`, `NO_HZ_IDLE`, `NO_HZ_FULL` and `PREEMPT` |
| **`kernel-selinux.enabled`** | true   | Selinux is enabled on the node                            |
| **`kernel-version.full`**    | string | Full kernel version as reported by `/proc/sys/kernel/osrelease` (e.g. '4.5.6-7-g123abcde') |
| **`kernel-version.major`**   | string | First component of the kernel version (e.g. '4')          |
| **`kernel-version.minor`**   | string | Second component of the kernel version (e.g. '5')         |
| **`kernel-version.revision`**| string | Third component of the kernel version (e.g. '6')          |

The kernel label source is configurable, see
[worker configuration](nfd-worker.md#worker-configuration) and
[`sources.kernel`](../reference/worker-configuration-reference.md#sourceskernel)
configuration options for details.

### Memory

| Feature              | Value | Description                                               |
| --------------------| ----- | --------------------------------------------------------- |
| **`memory-numa`**    | true  | Multiple memory nodes i.e. NUMA architecture detected     |
| **`memory-nv.present`** | true | NVDIMM device(s) are present                              |
| **`memory-nv.dax`** | true  | NVDIMM region(s) configured in DAX mode are present        |
| **`memory-swap.enabled`** | true  | Swap is enabled on the node                          |

### Network

| Feature                        | Value | Description                                                     |
| ------------------------------| ----- | --------------------------------------------------------------- |
| **`network-sriov.capable`**   | true  | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled Network Interface Card(s) present |
| **`network-sriov.configured`**| true  | SR-IOV virtual functions have been configured                   |

### PCI

| Feature                                 | Value | Description                                                      |
| --------------------------------------- | ----- | ---------------------------------------------------------------- |
| **`pci-<device label>.present`**       | true  | PCI device is detected                                           |
| **`pci-<device label>.sriov.capable`** | true  | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled PCI device present |
|                                         |       |                                                                    |

`<device label>` is format is configurable and set to `<class>_<vendor>` by
default. For more more details about configuration of the pci labels, see
[`sources.pci`](../reference/worker-configuration-reference.md#sourcespci) options
and [worker configuration](nfd-worker.md#worker-configuration)
instructions.

### USB

| Feature     | Value | Description                                               |
| ----------- | ----- | --------------------------------------------------------- |
| **`usb-<device label>.present`** | true | USB device is detected                                     |

`<device label>` is format is configurable and set to
`<class>_<vendor>_<device>` by default. For more more details about
configuration of the usb labels, see
[`sources.usb`](../reference/worker-configuration-reference.md#sourcesusb) options
and [worker configuration](nfd-worker.md#worker-configuration)
instructions.

### Storage

| Feature                          | Value | Description                                                 |
| --------------------------------| ----- | ----------------------------------------------------------- |
| **`storage-nonrotationaldisk`** | true  | Non-rotational disk, like SSD, is present in the node        |

### System

| Feature                                 | Value  | Description                                                 |
| --------------------------------------- | ------ | ----------------------------------------------------------- |
| **`system-os_release.ID`**              | string | Operating system identifier                                  |
| **`system-os_release.VERSION_ID`**      | string | Operating system version identifier (e.g. '6.7')            |
| **`system-os_release.VERSION_ID.major`**| string | First component of the OS version id (e.g. '6')             |
| **`system-os_release.VERSION_ID.minor`**| string | Second component of the OS version id (e.g. '7')            |

### Custom

The custom label source is designed for creating
[user defined labels](#user-defined-labels). However, it has a few statically
defined built-in labels:

| Feature                      | Value | Description                                                 |
| ---------------------------- | ----- | ----------------------------------------------------------- |
| **`custom-rdma.capable`**    | true  | The node has an RDMA capable Network adapter                 |
| **`custom-rdma.enabled`**    | true  | The node has the needed RDMA modules loaded to run RDMA traffic |
|                              |       |                                                             |

## User defined labels

NFD has many extension points for creating vendor and application specific
labels. See the [customization guide](customization-guide.md) for
detailed documentation.

## Extended resources

NFD is able to create extended resources, see the
[NodeFeatureRule](custom-resources.md#nodefeaturerule) CRD and its
[extendedResources](#customization-guide.md#extendedresources) field for more
details.

Note that NFD is not a replacement for the usage of device plugins.

An example use-case for extended resources could be based on custom feature
(created e.g. with [feature files](#customization-guide.md#feature-files) that
exposes the node SGX EPC memory section size. This value will then be turned
into an extended resource of the node, allowing PODs to request that resource
and the Kubernetes scheduler to schedule such PODs to only those nodes which
have a sufficient capacity of said resource left.

<!-- Links -->
[klauspost-cpuid]: https://github.com/klauspost/cpuid#x86-cpu-instructions
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
[intel-sst]: https://www.intel.com/content/www/us/en/architecture-and-technology/speed-select-technology-article.html
[sriov]: http://www.intel.com/content/www/us/en/pci-express/pci-sig-sr-iov-primer-sr-iov-technology-paper.html
