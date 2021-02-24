---
title: "Feature Discovery"
layout: default
sort: 4
---

# Feature Discovery
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

Feature discovery in nfd-worker is performed by a set of separate modules
called feature sources. Most of them are specifically responsible for certain
domain of features (e.g. cpu). In addition there are two highly customizable
feature sources that work accross the system.

## Feature labels

Each discovered feature is advertised a label in the Kubernetes Node object.
The published node labels encode a few pieces of information:

- Namespace, (all built-in labels use `feature.node.kubernetes.io`)
- The source for each label (e.g. `cpu`).
- The name of the discovered feature as it appears in the underlying
  source, (e.g. `cpuid.AESNI` from cpu).
- The value of the discovered feature.

Feature label names adhere to the following pattern:

```
<namespace>/<source name>-<feature name>[.<attribute name>]
```

The last component (i.e. `attribute-name`) is optional, and only used if a
feature logically has sub-hierarchy, e.g. `sriov.capable` and
`sriov.configure` from the `network` source.

The `--sources` flag controls which sources to use for discovery.

*Note: Consecutive runs of nfd-worker will update the labels on a
given node. If features are not discovered on a consecutive run, the corresponding
label will be removed. This includes any restrictions placed on the consecutive run,
such as restricting discovered features with the --label-whitelist option.*

## Feature Sources

### CPU

The **cpu** feature source supports the following labels:

| Feature name            | Attribute          | Description                   |
| ----------------------- | ------------------ | ----------------------------- |
| cpuid                   | &lt;cpuid flag&gt; | CPU capability is supported
| hardware_multithreading |                    | Hardware multithreading, such as Intel HTT, enabled (number of logical CPUs is greater than physical CPUs)
| power                   | sst_bf.enabled     | Intel SST-BF ([Intel Speed Select Technology][intel-sst] - Base frequency) enabled
| [pstate][intel-pstate]  | turbo              | Set to 'true' if turbo frequencies are enabled in Intel pstate driver, set to 'false' if they have been disabled.
| [rdt][intel-rdt]        | RDTMON             | Intel RDT Monitoring Technology
|                         | RDTCMT             | Intel Cache Monitoring (CMT)
|                         | RDTMBM             | Intel Memory Bandwidth Monitoring (MBM)
|                         | RDTL3CA            | Intel L3 Cache Allocation Technology
|                         | RDTL2CA            | Intel L2 Cache Allocation Technology
|                         | RDTMBA             | Intel Memory Bandwidth Allocation (MBA) Technology

The (sub-)set of CPUID attributes to publish is configurable via the
`attributeBlacklist` and `attributeWhitelist` cpuid options of the cpu source.
If whitelist is specified, only whitelisted attributes will be published. With
blacklist, only blacklisted attributes are filtered out. `attributeWhitelist`
has priority over `attributeBlacklist`.  For examples and more information
about configurability, see [configuration](deployment-and-usage#configuration).
By default, the following CPUID flags have been blacklisted:
BMI1, BMI2, CLMUL, CMOV, CX16, ERMS, F16C, HTT, LZCNT, MMX, MMXEXT, NX, POPCNT,
RDRAND, RDSEED, RDTSCP, SGX, SSE, SSE2, SSE3, SSE4.1, SSE4.2 and SSSE3.

**NOTE** The cpuid features advertise *supported* CPU capabilities, that is, a
capability might be supported but not enabled.

#### X86 CPUID Attributes (Partial List)

| Attribute | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| ADX       | Multi-Precision Add-Carry Instruction Extensions (ADX)
| AESNI     | Advanced Encryption Standard (AES) New Instructions (AES-NI)
| AVX       | Advanced Vector Extensions (AVX)
| AVX2      | Advanced Vector Extensions 2 (AVX2)

See the full list in [github.com/klauspost/cpuid][klauspost-cpuid].

#### Arm CPUID Attribute (Partial List)

| Attribute | Description                                                      |
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

#### Arm64 CPUID Attribute (Partial List)

| Attribute | Description                                                      |
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

### Custom

The Custom feature source allows the user to define features based on a mix of
predefined rules.  A rule is provided input witch affects its process of
matching for a defined feature. The rules are specified in the
nfd-worker configuration file. See
[configuration](deployment-and-usage.md#configuration) for instructions and
examples how to set-up and manage the worker configuration.

To aid in making Custom Features clearer, we define a general and a per rule
nomenclature, keeping things as consistent as possible.

#### Additional configuration directory

Additionally to the rules defined in the nfd-worker configuration file, the
Custom feature can read more configuration files located in the
`/etc/kubernetes/node-feature-discovery/custom.d/` directory. This makes more
dynamic and flexible configuration easier. This directory must be available
inside the NFD worker container, so Volumes and VolumeMounts must be used for
mounting e.g. ConfigMap(s). The example deployment manifests provide an example
(commented out) for providing Custom configuration with an additional
ConfigMap, mounted into the `custom.d` directory.

#### General Nomenclature & Definitions

```
Rule        :Represents a matching logic that is used to match on a feature.
Rule Input  :The input a Rule is provided. This determines how a Rule performs the match operation.
Matcher     :A composition of Rules, each Matcher may be composed of at most one instance of each Rule.
```

#### Custom Features Format (using the Nomenclature defined above)

Rules are specified under `sources.custom` in the nfd-worker configuration
file.

```yaml
sources:
  custom:
  - name: <feature name>
    value: <optional feature value, defaults to "true">
    matchOn:
    - <Rule-1>: <Rule-1 Input>
      [<Rule-2>: <Rule-2 Input>]
    - <Matcher-2>
    - ...
    - ...
    - <Matcher-N>
  - <custom feature 2>
  - ...
  - ...
  - <custom feature M>
```

#### Matching process

Specifying Rules to match on a feature is done by providing a list of Matchers.
Each Matcher contains one or more Rules.

Logical _OR_ is performed between Matchers and logical _AND_ is performed
between Rules of a given Matcher.

#### Rules

##### PciId Rule

###### Nomenclature

```
Attribute   :A PCI attribute.
Element     :An identifier of the PCI attribute.
```

The PciId Rule allows matching the PCI devices in the system on the following
Attributes: `class`,`vendor` and `device`. A list of Elements is provided for
each Attribute.

###### Format

```yaml
pciId :
  class: [<class id>, ...]
  vendor: [<vendor id>,  ...]
  device: [<device id>, ...]
```

Matching is done by performing a logical _OR_ between Elements of an Attribute
and logical _AND_ between the specified Attributes for each PCI device in the
system.  At least one Attribute must be specified. Missing attributes will not
partake in the matching process.

##### UsbId Rule

###### Nomenclature

```
Attribute   :A USB attribute.
Element     :An identifier of the USB attribute.
```

The UsbId Rule allows matching the USB devices in the system on the following
Attributes: `class`,`vendor` and `device`. A list of Elements is provided for
each Attribute.

###### Format

```yaml
usbId :
  class: [<class id>, ...]
  vendor: [<vendor id>,  ...]
  device: [<device id>, ...]
```

Matching is done by performing a logical _OR_ between Elements of an Attribute
and logical _AND_ between the specified Attributes for each USB device in the
system.  At least one Attribute must be specified. Missing attributes will not
partake in the matching process.

##### LoadedKMod Rule

###### Nomenclature

```
Element     :A kernel module
```

The LoadedKMod Rule allows matching the loaded kernel modules in the system
against a provided list of Elements.

###### Format

```yaml
loadedKMod : [<kernel module>, ...]
```

Matching is done by performing logical _AND_ for each provided Element, i.e
the Rule will match if all provided Elements (kernel modules) are loaded in the
system.

##### CpuId Rule

###### Nomenclature

```
Element     :A CPUID flag
```

The Rule allows matching the available CPUID flags in the system against a
provided list of Elements.

###### Format

```yaml
cpuId : [<CPUID flag string>, ...]
```

Matching is done by performing logical _AND_ for each provided Element, i.e the
Rule will match if all provided Elements (CPUID flag strings) are available in
the system.

##### Kconfig Rule

###### Nomenclature

```
Element     :A Kconfig option
```

The Rule allows matching the kconfig options in the system against a provided
list of Elements.

###### Format

```yaml
kConfig: [<kernel config option ('y' or 'm') or '=<value>'>, ...]
```

Matching is done by performing logical _AND_ for each provided Element, i.e the
Rule will match if all provided Elements (kernel config options) are enabled
(`y` or `m`) or matching `=<value>` in the kernel.

##### Nodename Rule

###### Nomenclature

```
Element     :A nodename regexp pattern
```

The Rule allows matching the node's name against a provided list of Elements.

###### Format

```yaml
nodename: [ <nodename regexp pattern>, ... ]
```

Matching is done by performing logical _OR_ for each provided Element, i.e the
Rule will match if one of the provided Elements (nodename regexp pattern)
matches the node's name.

#### Example

```yaml
custom:
  - name: "my.kernel.feature"
    matchOn:
      - loadedKMod: ["kmod1", "kmod2"]
  - name: "my.pci.feature"
    matchOn:
      - pciId:
          vendor: ["15b3"]
          device: ["1014", "1017"]
  - name: "my.usb.feature"
    matchOn:
      - usbId:
          vendor: ["1d6b"]
          device: ["0003"]
  - name: "my.combined.feature"
    matchOn:
      - loadedKMod : ["vendor_kmod1", "vendor_kmod2"]
        pciId:
          vendor: ["15b3"]
          device: ["1014", "1017"]
  - name: "my.accumulated.feature"
    matchOn:
      - loadedKMod : ["some_kmod1", "some_kmod2"]
      - pciId:
          vendor: ["15b3"]
          device: ["1014", "1017"]
  - name: "my.kernel.featureneedscpu"
    matchOn:
      - kConfig: ["KVM_INTEL"]
      - cpuId: ["VMX"]
  - name: "my.kernel.modulecompiler"
    matchOn:
      - kConfig: ["GCC_VERSION=100101"]
        loadedKMod: ["kmod1"]
  - name: "my.datacenter"
    value: "datacenter-1"
    matchOn:
      - nodename: [ "node-datacenter1-rack.*-server.*" ]
```

__In the example above:__

- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.kernel.feature=true` if the node has
  `kmod1` _AND_ `kmod2` kernel modules loaded.
- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.pci.feature=true` if the node contains
  a PCI device with a PCI vendor ID of `15b3` _AND_ PCI device ID of `1014` _OR_
  `1017`.
- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.usb.feature=true` if the node contains
  a USB device with a USB vendor ID of `1d6b` _AND_ USB device ID of `0003`.
- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.combined.feature=true` if
  `vendor_kmod1` _AND_ `vendor_kmod2` kernel modules are loaded __AND__ the node
  contains a PCI device
  with a PCI vendor ID of `15b3` _AND_ PCI device ID of `1014` _or_ `1017`.
- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.accumulated.feature=true` if
  `some_kmod1` _AND_ `some_kmod2` kernel modules are loaded __OR__ the node
  contains a PCI device
  with a PCI vendor ID of `15b3` _AND_ PCI device ID of `1014` _OR_ `1017`.
- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.kernel.featureneedscpu=true` if
  `KVM_INTEL` kernel config is enabled __AND__ the node CPU supports `VMX`
  virtual machine extensions
- A node would contain the label:
  `feature.node.kubernetes.io/custom-my.kernel.modulecompiler=true` if the
  in-tree `kmod1` kernel module is loaded __AND__ it's built with
  `GCC_VERSION=100101`.
- A node would contain the label:
  `feature.node.kubernetes.io/my.datacenter=datacenter-1` if the node's name
  matches the `node-datacenter1-rack.*-server.*` pattern, e.g.
  `node-datacenter1-rack2-server42`

#### Statically defined features

Some feature labels which are common and generic are defined statically in the
`custom` feature source.  A user may add additional Matchers to these feature
labels by defining them in the `nfd-worker` configuration file.

| Feature | Attribute | Description |
| ------- | --------- | -----------|
| rdma  | capable | The node has an RDMA capable Network adapter |
| rdma | enabled | The node has the needed RDMA modules loaded to run RDMA traffic |

### IOMMU

The **iommu** feature source supports the following labels:

| Feature name   | Description                                                 |
| :------------: | :---------------------------------------------------------: |
| enabled        | IOMMU is present and enabled in the kernel

### Kernel

The **kernel** feature source supports the following labels:

| Feature | Attribute           | Description                                  |
| ------- | ------------------- | -------------------------------------------- |
| config  | &lt;option name&gt; | Kernel config option is enabled (set 'y' or 'm').<br> Default options are `NO_HZ`, `NO_HZ_IDLE`, `NO_HZ_FULL` and `PREEMPT`
| selinux | enabled             | Selinux is enabled on the node
| version | full                | Full kernel version as reported by `/proc/sys/kernel/osrelease` (e.g. '4.5.6-7-g123abcde')
|         | major               | First component of the kernel version (e.g. '4')
|         | minor               | Second component of the kernel version (e.g. '5')
|         | revision            | Third component of the kernel version (e.g. '6')

Kernel config file to use, and, the set of config options to be detected are
configurable.
See [configuration](deployment-and-usage#configuration) for
more information.

### Memory

The **memory** feature source supports the following labels:

| Feature | Attribute | Description                                            |
| ------- | --------- | ------------------------------------------------------ |
| numa    |           | Multiple memory nodes i.e. NUMA architecture detected
| nv      | present   | NVDIMM device(s) are present
| nv      | dax       | NVDIMM region(s) configured in DAX mode are present

### Network

The **network** feature source supports the following labels:

| Feature | Attribute  | Description                                           |
| ------- | ---------- | ----------------------------------------------------- |
| sriov   | capable    | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled Network Interface Card(s) present
|         | configured | SR-IOV virtual functions have been configured

### PCI

The **pci** feature source supports the following labels:

| Feature              | Attribute     | Description                           |
| -------------------- | ------------- | ------------------------------------- |
| &lt;device label&gt; | present       | PCI device is detected
| &lt;device label&gt; | sriov.capable | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled PCI device present

`<device label>` is composed of raw PCI IDs, separated by underscores.  The set
of fields used in `<device label>` is configurable, valid fields being `class`,
`vendor`, `device`, `subsystem_vendor` and `subsystem_device`.  Defaults are
`class` and `vendor`. An example label using the default label fields:

```
feature.node.kubernetes.io/pci-1200_8086.present=true
```

Also  the set of PCI device classes that the feature source detects is
configurable. By default, device classes (0x)03, (0x)0b40 and (0x)12, i.e.
GPUs, co-processors and accelerator cards are detected.

### USB

The **usb** feature source supports the following labels:

| Feature              | Attribute     | Description                           |
| -------------------- | ------------- | ------------------------------------- |
| &lt;device label&gt; | present       | USB device is detected

`<device label>` is composed of raw USB IDs, separated by underscores.  The set
of fields used in `<device label>` is configurable, valid fields being `class`,
`vendor`, and `device`.  Defaults are `class`, `vendor` and `device`. An
example label using the default label fields:

```
feature.node.kubernetes.io/usb-fe_1a6e_089a.present=true
```

See [configuration](deployment-and-usage#configuration) for more information on NFD
config.

### Storage

The **storage** feature source supports the following labels:

| Feature name       | Description                                             |
| ------------------ | ------------------------------------------------------- |
| nonrotationaldisk  | Non-rotational disk, like SSD, is present in the node

### System

The **system** feature source supports the following labels:

| Feature     | Attribute        | Description                                 |
| ----------- | ---------------- | --------------------------------------------|
| os_release  | ID               | Operating system identifier
|             | VERSION_ID       | Operating system version identifier (e.g. '6.7')
|             | VERSION_ID.major | First component of the OS version id (e.g. '6')
|             | VERSION_ID.minor | Second component of the OS version id (e.g. '7')

### Local -- User-specific Features

NFD has a special feature source named *local* which is designed for getting
the labels from user-specific feature detector. It provides a mechanism for
users to implement custom feature sources in a pluggable way, without modifying
nfd source code or Docker images. The local feature source can be used to
advertise new user-specific features, and, for overriding labels created by the
other feature sources.

The *local* feature source gets its labels by two different ways:

- It tries to execute files found under
  `/etc/kubernetes/node-feature-discovery/source.d/` directory. The hook files
  must be executable and they are supposed to print all discovered features in
  `stdout`, one per line. With ELF binaries static linking is recommended as
  the selection of system libraries available in the NFD release image is very
  limited. Other runtimes currently supported by the NFD stock image are bash
  and perl.
- It reads files found under
  `/etc/kubernetes/node-feature-discovery/features.d/` directory. The file
  content is expected to be similar to the hook output (described above).

These directories must be available inside the Docker image so Volumes and
VolumeMounts must be used if standard NFD images are used. The given template
files mount by default the `source.d` and the `features.d` directories
respectively from `/etc/kubernetes/node-feature-discovery/source.d/` and
`/etc/kubernetes/node-feature-discovery/features.d/` from the host. You should
update them to match your needs.

In both cases, the labels can be binary or non binary, using either `<name>` or
`<name>=<value>` format.

Unlike the other feature sources, the name of the file, instead of the name of
the feature source (that would be `local` in this case), is used as a prefix in
the label name, normally. However, if the `<name>` of the label starts with a
slash (`/`) it is used as the label name as is, without any additional prefix.
This makes it possible for the user to fully control the feature label names,
e.g. for overriding labels created by other feature sources.

You can also override the default namespace of your labels using this format:
`<namespace>/<name>[=<value>]`. You must whitelist your namespace using the
`--extra-label-ns` option on the master. In this case, the name of the
file will not be added to the label name. For example, if you want to add the
label `my.namespace.org/my-label=value`, your hook output or file must contains
`my.namespace.org/my-label=value` and you must add
`--extra-label-ns=my.namespace.org` on the master command line.

`stderr` output of the hooks is propagated to NFD log so it can be used for
debugging and logging.

#### Injecting Labels from Other Pods

One use case for the hooks and/or feature files is detecting features in other
Pods outside NFD, e.g. in Kubernetes device plugins. It is possible to mount
the `source.d` and/or `features.d` directories common with the NFD Pod and
deploy the custom hooks/features there. NFD will periodically scan the
directories and run any hooks and read any feature files it finds. The
[example nfd-worker deployment template](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{ site.release }}/nfd-worker-daemonset.yaml.template#L69)
contains `hostPath` mounts for `sources.d` and `features.d` directories. By
using the same mounts in the secondary Pod (e.g. device plugin) you have
created a shared area for delivering hooks and feature files to NFD.

#### A Hook Example

User has a shell script
`/etc/kubernetes/node-feature-discovery/source.d/my-source` which has the
following `stdout` output:

```
MY_FEATURE_1
MY_FEATURE_2=myvalue
/override_source-OVERRIDE_BOOL
/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```

which, in turn, will translate into the following node labels:

```
feature.node.kubernetes.io/my-source-MY_FEATURE_1=true
feature.node.kubernetes.io/my-source-MY_FEATURE_2=myvalue
feature.node.kubernetes.io/override_source-OVERRIDE_BOOL=true
feature.node.kubernetes.io/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```

#### A File Example

User has a file `/etc/kubernetes/node-feature-discovery/features.d/my-source`
which contains the following lines:

```
MY_FEATURE_1
MY_FEATURE_2=myvalue
/override_source-OVERRIDE_BOOL
/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```

which, in turn, will translate into the following node labels:

```
feature.node.kubernetes.io/my-source-MY_FEATURE_1=true
feature.node.kubernetes.io/my-source-MY_FEATURE_2=myvalue
feature.node.kubernetes.io/override_source-OVERRIDE_BOOL=true
feature.node.kubernetes.io/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```

NFD tries to run any regular files found from the hooks directory. Any
additional data files your hook might need (e.g. a configuration file) should
be placed in a separate directory in order to avoid NFD unnecessarily trying to
execute these. You can use a subdirectory under the hooks directory, for
example `/etc/kubernetes/node-feature-discovery/source.d/conf/`.

**NOTE!** NFD will blindly run any executables placed/mounted in the hooks
directory. It is the user's responsibility to review the hooks for e.g.
possible security implications.

**NOTE!** Be careful when creating and/or updating hook or feature files while
NFD is running. In order to avoid race conditions you should write into a
temporary file (outside the `source.d` and `features.d` directories), and,
atomically create/update the original file by doing a filesystem move
operation.

## Extended resources

This feature is experimental and by no means a replacement for the usage of
device plugins.

Labels which have integer values, can be promoted to Kubernetes extended
resources by listing them to the master `--resource-labels` command line flag.
These labels won't then show in the node label section, they will appear only
as extended resources.

An example use-case for the extended resources could be based on a hook which
creates a label for the node SGX EPC memory section size. By giving the name of
that label in the `--resource-labels` flag, that value will then turn into an
extended resource of the node, allowing PODs to request that resource and the
Kubernetes scheduler to schedule such PODs to only those nodes which have a
sufficient capacity of said resource left.

Similar to labels, the default namespace `feature.node.kubernetes.io` is
automatically prefixed to the extended resource, if the promoted label doesn't
have a namespace.

Example usage of the command line arguments, using a new namespace:
`nfd-master --resource-labels=my_source-my.feature,sgx.some.ns/epc --extra-label-ns=sgx.some.ns`

The above would result in following extended resources provided that related
labels exist:

```
  sgx.some.ns/epc: <label value>
  feature.node.kubernetes.io/my_source-my.feature: <label value>
```

<!-- Links -->
[klauspost-cpuid]: https://github.com/klauspost/cpuid#x86-cpu-instructions
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
[intel-sst]: https://www.intel.com/content/www/us/en/architecture-and-technology/speed-select-technology-article.html
[sriov]: http://www.intel.com/content/www/us/en/pci-express/pci-sig-sr-iov-primer-sr-iov-technology-paper.html
