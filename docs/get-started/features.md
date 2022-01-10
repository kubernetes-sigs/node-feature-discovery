---
title: "Feature labels"
layout: default
sort: 4
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
[`core.labelSources`](../advanced/worker-configuration-reference#corelabelsources)
configuration option (or
[`-label-sources`](../advanced/worker-commandline-reference#-label-sources)
flag) of nfd-worker controls which sources to enable for label generation.

All built-in labels use the `feature.node.kubernetes.io` label namespace and
have the following format.

```plaintext
feature.node.kubernetes.io/<feature> = <value>
```

*Note: Consecutive runs of nfd-worker will update the labels on a
given node. If features are not discovered on a consecutive run, the corresponding
label will be removed. This includes any restrictions placed on the consecutive run,
such as restricting discovered features with the -label-whitelist option.*

### CPU

| Feature name            | Value        | Description
| ----------------------- | ------------ | -----------
| **`cpu-cpuid.<cpuid-flag>`**      | true   | CPU capability is supported. **NOTE:** the capability might be supported but not enabled.
| **`cpu-hardware_multithreading`** | true   | Hardware multithreading, such as Intel HTT, enabled (number of logical CPUs is greater than physical CPUs)
| **`cpu-power.sst_bf.enabled`**    | true   | Intel SST-BF ([Intel Speed Select Technology][intel-sst] - Base frequency) enabled
| **`cpu-pstate.status`**           | string | The status of the [Intel pstate][intel-pstate] driver when in use and enabled, either 'active' or 'passive'.
| **`cpu-pstate.turbo`**            | bool   | Set to 'true' if turbo frequencies are enabled in Intel pstate driver, set to 'false' if they have been disabled.
| **`cpu-pstate.scaling_governor`** | string | The value of the Intel pstate scaling_governor when in use, either 'powersave' or 'performance'.
| **`cpu-cstate.enabled`**          | bool   | Set to 'true' if cstates are set in the intel_idle driver, otherwise set to 'false'. Unset if intel_idle cpuidle driver is not active.
| **`cpu-rdt.<rdt-flag>`**          | true   | [Intel RDT][intel-rdt] capability is supported. See [RDT flags](#intel-rdt-flags) for details.
| **`cpu-sgx.enabled`**             | true   | Set to 'true' if Intel SGX is enabled in BIOS (based a non-zero sum value of SGX EPC section sizes).

The CPU label source is configurable, see
[worker configuration](deployment-and-usage#worker-configuration) and
[`sources.cpu`](../advanced/worker-configuration-reference#sourcescpu)
configuration options for details.

#### X86 CPUID flags (partial list)

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| ADX       | Multi-Precision Add-Carry Instruction Extensions (ADX)
| AESNI     | Advanced Encryption Standard (AES) New Instructions (AES-NI)
| AVX       | Advanced Vector Extensions (AVX)
| AVX2      | Advanced Vector Extensions 2 (AVX2)

By default, the following CPUID flags have been blacklisted: BMI1, BMI2, CLMUL,
CMOV, CX16, ERMS, F16C, HTT, LZCNT, MMX, MMXEXT, NX, POPCNT, RDRAND, RDSEED,
RDTSCP, SGX, SSE, SSE2, SSE3, SSE4, SSE42 and SSSE3. See
[`sources.cpu`](../advanced/worker-configuration-reference#sourcescpu)
configuration options to change the behavior.

See the full list in [github.com/klauspost/cpuid][klauspost-cpuid].

#### Arm CPUID flags (partial list)

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| IDIVA     | Integer divide instructions available in ARM mode
| IDIVT     | Integer divide instructions available in Thumb mode
| THUMB     | Thumb instructions
| FASTMUL   | Fast multiplication
| VFP       | Vector floating point instruction extension (VFP)
| VFPv3     | Vector floating point extension v3
| VFPv4     | Vector floating point extension v4
| VFPD32    | VFP with 32 D-registers
| HALF      | Half-word loads and stores
| EDSP      | DSP extensions
| NEON      | NEON SIMD instructions
| LPAE      | Large Physical Address Extensions

#### Arm64 CPUID flags (partial list)

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| AES       | Announcing the Advanced Encryption Standard
| EVSTRM    | Event Stream Frequency Features
| FPHP      | Half Precision(16bit) Floating Point Data Processing Instructions
| ASIMDHP   | Half Precision(16bit) Asimd Data Processing Instructions
| ATOMICS   | Atomic Instructions to the A64
| ASIMRDM   | Support for Rounding Double Multiply Add/Subtract
| PMULL     | Optional Cryptographic and CRC32 Instructions
| JSCVT     | Perform Conversion to Match Javascript
| DCPOP     | Persistent Memory Support

#### Intel RDT flags

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| RDTMON    | Intel RDT Monitoring Technology
| RDTCMT    | Intel Cache Monitoring (CMT)
| RDTMBM    | Intel Memory Bandwidth Monitoring (MBM)
| RDTL3CA   | Intel L3 Cache Allocation Technology
| RDTl2CA   | Intel L2 Cache Allocation Technology
| RDTMBA    | Intel Memory Bandwidth Allocation (MBA) Technology

### IOMMU (deprecated)

| Feature             | Value | Description
| ------------------- | ----- | -----------
| **`iommu.enabled`** | true  | IOMMU is present and enabled in the kernel

**DEPRECATED**: The **iommu** source is deprecated and not enabled by default.

### Kernel

| Feature | Value  | Description
| ------- | ------ | -----------
| **`kernel-config.<option>`**  | true   | Kernel config option is enabled (set 'y' or 'm'). Default options are `NO_HZ`, `NO_HZ_IDLE`, `NO_HZ_FULL` and `PREEMPT`
| **`kernel-selinux.enabled`**  | true   | Selinux is enabled on the node
| **`kernel-version.full`**     | string | Full kernel version as reported by `/proc/sys/kernel/osrelease` (e.g. '4.5.6-7-g123abcde')
| **`kernel-version.major`**    | string | First component of the kernel version (e.g. '4')
| **`kernel-version.minor`**    | string | Second component of the kernel version (e.g. '5')
| **`kernel-version.revision`** | string | Third component of the kernel version (e.g. '6')

The kernel label source is configurable, see
[worker configuration](deployment-and-usage#worker-configuration) and
[`sources.kernel`](../advanced/worker-configuration-reference#sourceskernel)
configuration options for details.

### Memory

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`memory-numa`**       | true | Multiple memory nodes i.e. NUMA architecture detected
| **`memory-nv.present`** | true | NVDIMM device(s) are present
| **`memory-nv.dax`**     | true | NVDIMM region(s) configured in DAX mode are present

### Network

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`network-sriov.capable`**    | true | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled Network Interface Card(s) present
| **`network-sriov.configured`** | true | SR-IOV virtual functions have been configured

### PCI

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`pci-<device label>.present`**       | true | PCI device is detected
| **`pci-<device label>.sriov.capable`** | true | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled PCI device present

`<device label>` is format is configurable and set to `<class>_<vendor>` by
default. For more more details about configuration of the pci labels, see
[`sources.pci`](../advanced/worker-configuration-reference#sourcespci) options
and [worker configuration](deployment-and-usage#worker-configuration)
instructions.

### USB

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`usb-<device label>.present`** | true | USB device is detected

`<device label>` is format is configurable and set to
`<class>_<vendor>_<device>` by default. For more more details about
configuration of the usb labels, see
[`sources.usb`](../advanced/worker-configuration-reference#sourcesusb) options
and [worker configuration](deployment-and-usage#worker-configuration)
instructions.

### Storage

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`storage-nonrotationaldisk`** | true | Non-rotational disk, like SSD, is present in the node

### System

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`system-os_release.ID`**               | string | Operating system identifier
| **`system-os_release.VERSION_ID`**       | string |Operating system version identifier (e.g. '6.7')
| **`system-os_release.VERSION_ID.major`** | string |First component of the OS version id (e.g. '6')
| **`system-os_release.VERSION_ID.minor`** | string | Second component of the OS version id (e.g. '7')

### Custom

The custom label source is designed for creating
[user defined labels](#user-defined-labels). However, it has a few statically
defined built-in labels:

| Feature     | Value | Description
| ----------- | ----- | -----------
| **`custom-rdma.capable`** | true | The node has an RDMA capable Network adapter |
| **`custom-rdma.enabled`** | true | The node has the needed RDMA modules loaded to run RDMA traffic |

## User defined labels

NFD has many extension points for creating vendor and application specific
labels. See the [customization guide](../advanced/customization-guide.md) for
detailed documentation.

## Extended resources

This feature is experimental and by no means a replacement for the usage of
device plugins.

Labels which have integer values, can be promoted to Kubernetes extended
resources by listing them to the master `-resource-labels` command line flag.
These labels won't then show in the node label section, they will appear only
as extended resources.

An example use-case for the extended resources could be based on a hook which
creates a label for the node SGX EPC memory section size. By giving the name of
that label in the `-resource-labels` flag, that value will then turn into an
extended resource of the node, allowing PODs to request that resource and the
Kubernetes scheduler to schedule such PODs to only those nodes which have a
sufficient capacity of said resource left.

Similar to labels, the default namespace `feature.node.kubernetes.io` is
automatically prefixed to the extended resource, if the promoted label doesn't
have a namespace.

Example usage of the command line arguments, using a new namespace:
`nfd-master -resource-labels=my_source-my.feature,sgx.some.ns/epc -extra-label-ns=sgx.some.ns`

The above would result in following extended resources provided that related
labels exist:

```plaintext
  sgx.some.ns/epc: <label value>
  feature.node.kubernetes.io/my_source-my.feature: <label value>
```

<!-- Links -->
[klauspost-cpuid]: https://github.com/klauspost/cpuid#x86-cpu-instructions
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
[intel-sst]: https://www.intel.com/content/www/us/en/architecture-and-technology/speed-select-technology-article.html
[sriov]: http://www.intel.com/content/www/us/en/pci-express/pci-sig-sr-iov-primer-sr-iov-technology-paper.html
