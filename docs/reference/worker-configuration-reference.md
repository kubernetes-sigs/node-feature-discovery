---
title: "Worker config reference"
parent: "Reference"
layout: default
nav_order: 4
---

# Configuration file reference of nfd-worker
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

See the
[sample configuration file](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/worker-config/nfd-worker.conf.example)
for a full example configuration.

## core

The `core` section contains common configuration settings that are not specific
to any particular feature source.

### core.sleepInterval

`core.sleepInterval` specifies the interval between consecutive passes of
feature (re-)detection, and thus also the interval between node re-labeling. A
non-positive value implies infinite sleep interval, i.e. no re-detection or
re-labeling is done.

Default: `60s`

Example:

```yaml
core:
  sleepInterval: 60s
```

### core.featureSources

`core.featureSources` specifies the list of enabled feature sources. A special
value `all` enables all sources. Prefixing a source name with `-` indicates
that the source will be disabled instead - this is only meaningful when used in
conjunction with `all`. This option allows completely disabling the feature
detection so that neither standard feature labels are generated nor the raw
feature data is available for custom rule processing.

Default: `[all]`

Example:

```yaml
core:
  # Enable all but cpu and local sources
  featureSources:
    - "all"
    - "-cpu"
    - "-local"
```

```yaml
core:
  # Enable only cpu and local sources
  featureSources:
    - "cpu"
    - "local"
```

### core.labelSources

`core.labelSources` specifies the list of enabled label sources. A special
value `all` enables all sources. Prefixing a source name with `-` indicates
that the source will be disabled instead - this is only meaningful when used in
conjunction with `all`. This configuration option affects the generation of
node labels but not the actual discovery of the underlying feature data that is
used e.g. in custom/`NodeFeatureRule` rules.

> **NOTE:** Overridden by the `-label-sources` command line flag and the
> `core.sources` configurations option (if either of them is specified).

Default: `[all]`

Example:

```yaml
core:
  # Enable all but cpu and system sources
  labelSources:
    - "all"
    - "-cpu"
    - "-system"
```

```yaml
core:
  # Enable only cpu and system sources
  labelSources:
    - "cpu"
    - "system"
```

### core.sources

**DEPRECATED**: use [`core.labelSources`](#core.labelSources) instead.

> **NOTE:** `core.sources` takes precedence over the `core.labelSources`
> configuration file option.

### core.labelWhiteList

`core.labelWhiteList` specifies a regular expression for filtering feature
labels based on the label name. Non-matching labels are not published.

> **NOTE:** The regular expression is only matches against the "basename" part
> of the label, i.e. to the part of the name after '/'. The label prefix (or
> namespace) is omitted.

Default: `null`

Example:

```yaml
core:
  labelWhiteList: '^cpu-cpuid'
```

### core.noPublish

Setting `core.noPublish` to `true` disables all communication with the
nfd-master and the Kubernetes API server. It is effectively a "dry-run" option.
NFD-Worker runs feature detection normally, but no labeling requests are sent
to nfd-master and no [NodeFeature](../usage/custom-resources.md#nodefeature)
objects are created or updated in the API server.

> **NOTE:** Overridden by the
> [`-no-publish`](worker-commandline-reference.md#-no-publish)
> command line flag (if specified).

Default: `false`

Example:

```yaml
core:
  noPublish: true
```

### core.noOwnerRefs

Setting `core.noOwnerRefs` to `true` disables setting the owner references
of the NodeFeature object created by the nfd-worker.

> **NOTE:** Overridden by the
> [`-no-owner-refs`](worker-commandline-reference.md#-no-owner-refs)
> command line flag (if specified).

Default: `false`

Example:

```yaml
core:
  noOwnerRefs: true
```

### core.klog

The following options specify the logger configuration.

> **NOTE:** The logger options can also be specified via command line flags
> which take precedence over any corresponding config file options.

#### core.klog.addDirHeader

If true, adds the file directory to the header of the log messages.

Default: `false`

#### core.klog.alsologtostderr

Log to standard error as well as files.

Default: `false`

#### core.klog.logBacktraceAt

When logging hits line file:N, emit a stack trace.

Default: *empty*

#### core.klog.logDir

If non-empty, write log files in this directory.

Default: *empty*

#### core.klog.logFile

If non-empty, use this log file.

Default: *empty*

#### core.klog.logFileMaxSize

Defines the maximum size a log file can grow to. Unit is megabytes. If the
value is 0, the maximum file size is unlimited.

Default: `1800`

#### core.klog.logtostderr

Log to standard error instead of files

Default: `true`

#### core.klog.skipHeaders

If true, avoid header prefixes in the log messages.

Default: `false`

#### core.klog.skipLogHeaders

If true, avoid headers when opening log files.

Default: `false`

#### core.klog.stderrthreshold

Logs at or above this threshold go to stderr (default 2)

#### core.klog.v

Number for the log level verbosity.

Default: `0`

#### core.klog.vmodule

Comma-separated list of `pattern=N` settings for file-filtered logging.

Default: *empty*

## sources

The `sources` section contains feature source specific configuration parameters.

### sources.cpu

#### sources.cpu.cpuid

##### sources.cpu.cpuid.attributeBlacklist

Prevent publishing cpuid features listed in this option.

> **NOTE:** overridden by `sources.cpu.cpuid.attributeWhitelist` (if specified)

Default: `[AVX10, BMI1, BMI2, CLMUL, CMOV, CX16, ERMS, F16C, HTT, LZCNT, MMX, MMXEXT,
NX, POPCNT, RDRAND, RDSEED, RDTSCP, SGX, SGXLC, SSE, SSE2, SSE3, SSE4.1,
SSE4.2, SSSE3, TDX_GUEST]`

Example:

```yaml
sources:
  cpu:
    cpuid:
      attributeBlacklist: [MMX, MMXEXT]
```

##### sources.cpu.cpuid.attributeWhitelist

Only publish the cpuid features listed in this option.

> **NOTE:** takes precedence over `sources.cpu.cpuid.attributeBlacklist`

Default: *empty*

Example:

```yaml
sources:
  cpu:
    cpuid:
      attributeWhitelist: [AVX512BW, AVX512CD, AVX512DQ, AVX512F, AVX512VL]
```

### sources.kernel

#### sources.kernel.kconfigFile

Path of the kernel config file. If empty, NFD runs a search in the well-known
standard locations.

Default: *empty*

Example:

```yaml
sources:
  kernel:
    kconfigFile: "/path/to/kconfig"
```

#### sources.kernel.configOpts

Kernel configuration options to publish as feature labels.

Default: `[NO_HZ, NO_HZ_IDLE, NO_HZ_FULL, PREEMPT]`

Example:

```yaml
sources:
  kernel:
    configOpts: [NO_HZ, X86, DMI]
```

### sources.local

### sources.pci

#### sources.pci.deviceClassWhitelist

List of PCI [device class](https://pci-ids.ucw.cz/read/PD) IDs for which to
publish a label. Can be specified as a main class only (e.g. `03`) or full
class-subclass combination (e.g. `0300`) - the former implies that all
subclasses are accepted.  The format of the labels can be further configured
with [deviceLabelFields](#sources.pci.deviceLabelFields).

Default: `["03", "0b40", "12"]`

Example:

```yaml
sources:
  pci:
    deviceClassWhitelist: ["0200", "03"]
```

#### sources.pci.deviceLabelFields

The set of PCI ID fields to use when constructing the name of the feature
label. Valid fields are `class`, `vendor`, `device`, `subsystem_vendor` and
`subsystem_device`.

Default: `[class, vendor]`

Example:

```yaml
sources:
  pci:
    deviceLabelFields: [class, vendor, device]
```

With the example config above NFD would publish labels like:
`feature.node.kubernetes.io/pci-<class-id>_<vendor-id>_<device-id>.present=true`

### sources.usb

#### sources.usb.deviceClassWhitelist

List of USB [device class](https://www.usb.org/defined-class-codes) IDs for
which to publish a feature label. The format of the labels can be further
configured with [deviceLabelFields](#sources.usb.deviceLabelFields).

Default: `["0e", "ef", "fe", "ff"]`

Example:

```yaml
sources:
  usb:
    deviceClassWhitelist: ["ef", "ff"]
```

#### sources.usb.deviceLabelFields

The set of USB ID fields from which to compose the name of the feature label.
Valid fields are `class`, `vendor`, `device` and `serial`.

Default: `[class, vendor, device]`

Example:

```yaml
sources:
  pci:
    deviceLabelFields: [class, vendor]
```

With the example config above NFD would publish labels like:
`feature.node.kubernetes.io/usb-<class-id>_<vendor-id>.present=true`

### sources.custom

List of rules to process in the custom feature source to create user-specific
labels. Refer to the documentation of the
[custom feature source](../usage/customization-guide.md#custom-feature-source) for
details of the available rules and their configuration.

Default: *empty*

Example:

```yaml
sources:
  custom:
    - name: "my custom rule"
      labels:
        my-custom-feature: "true"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            e1000e: {op: Exists}
        - feature: pci.device
          matchExpressions:
            class: {op: In, value: ["0200"]}
            vendor: {op: In, value: ["8086"]}
```
