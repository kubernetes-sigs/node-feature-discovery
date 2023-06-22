---
title: "Customization guide"
layout: default
sort: 8
---

# Customization guide
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

## Overview

NFD provides multiple extension points for vendor and application specific
labeling:

- [`NodeFeature`](#nodefeature-custom-resource) (EXPERIMENTAL) objects can be
  used to communicate "raw" node features and node labeling requests to
  nfd-master.
- [`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects provide a way to
  deploy custom labeling rules via the Kubernetes API.
- [`local`](#local-feature-source) feature source of nfd-worker creates
  labels by executing hooks and reading files.
- [`custom`](#custom-feature-source) feature source of nfd-worker creates
  labels based on user-specified rules.

## NodeFeature custom resource

**EXPERIMENTAL**
NodeFeature objects provide a way for 3rd party extensions to advertise custom
features, both as "raw" features that serve as input to
[NodeFeatureRule](#nodefeaturerule-custom-resource) objects and as feature
labels directly.

Note that RBAC rules must be created for each extension for them to be able to
create and manipulate NodeFeature objects in their namespace.

Support for NodeFeature CRD API is enabled with the `-enable-nodefeature-api`
command line flag. This flag must be specified for both nfd-master and
nfd-worker as it will disable the gRPC communication between them.

### A NodeFeature example

Consider the following referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeature
metadata:
  labels:
    nfd.node.kubernetes.io/node-name: node-1
  name: vendor-features-for-node-1
spec:
  # Features for NodeFeatureRule matching
  features:
    flags:
      vendor.flags:
        elements:
          feature-x: {}
          feature-y: {}
    attributes:
      vendor.config:
        elements:
          setting-a: "auto"
          knob-b: "123"
    instances:
      vendor.devices:
        elements:
        - attributes:
            model: "dev-1000"
            vendor: "acme"
        - attributes:
            model: "dev-2000"
            vendor: "acme"
  # Labels to be created
  labels:
    vendor-feature.enabled: "true"
```

The object targets node named `node-1`. It lists two "flag type" features under
the `vendor.flags` domain, two "attribute type" features and under the
`vendor.config` domain and two "instance type" features under the
`vendor.devices` domain. These features will not be directly affecting the node
labels but they will be used as input when the
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects are evaluated.

In addition, the example requests directly the
`feature.node.kubenernetes.io/vendor-feature.enabled=true` node label to be
created.

The `nfd.node.kubernetes.io/node-name=<node-name>` must be in place for each
NodeFeature object as NFD uses it to determine the node which it is targeting.

### Feature types

Features are divided into three different types:

- **flag** features: a set of names without any associated values, e.g. CPUID
  flags or loaded kernel modules
- **attribute** features: a set of names each of which has a single value
  associated with it (essentially a map of key-value pairs), e.g. kernel config
  flags or os release information
- **instance** features: a list of instances, each of which has multiple
  attributes (key-value pairs of their own) associated with it, e.g. PCI or USB
  devices

## NodeFeatureRule custom resource

`NodeFeatureRule` objects provide an easy way to create vendor or application
specific labels and taints. It uses a flexible rule-based mechanism for creating
labels and optionally taints based on node features.

### A NodeFeatureRule example

Consider the following referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-sample-rule-object
spec:
  rules:
    - name: "my sample rule"
      labels:
        "my-sample-feature": "true"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            dummy: {op: Exists}
        - feature: kernel.config
          matchExpressions:
            X86: {op: In, value: ["y"]}
```

It specifies one rule which creates node label
`feature.node.kubenernetes.io/my-sample-feature=true` if both of the following
conditions are true (`matchFeatures` implements a logical AND over the
matchers):

- The `dummy` network driver module has been loaded
- X86 option in kernel config is set to `=y`

Create a `NodeFeatureRule` with a yaml file:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/examples/nodefeaturerule.yaml
```

Now, on X86 platforms the feature label appears after doing `modprobe dummy` on
a system and correspondingly the label is removed after `rmmod dummy`. Note a
re-labeling delay up to the sleep-interval of nfd-worker (1 minute by default).

See [Label rule format](#label-rule-format) for detailed description of
available fields and how to write labeling rules.

### NodeFeatureRule tainting feature

This feature is experimental.

In some circumstances, it is desirable to keep nodes with specialized hardware
away from running general workload and instead leave them for workloads that
need the specialized hardware. One way to achieve it is to taint the nodes with
the specialized hardware and add corresponding toleration to pods that require
the special hardware. NFD offers node tainting functionality which is disabled
by default. User can define one or more custom taints via the `taints` field of
the NodeFeatureRule CR. The same rule-based mechanism is applied here and the
NFD taints only rule matching nodes.

To enable the tainting feature, `--enable-taints` flag needs to be set to `true`.
If the flag `--enable-taints` is set to `false` (i.e. disabled), taints defined in
the NodeFeatureRule CR have no effect and will be ignored by the NFD master.

**NOTE**: Before enabling any taints, make sure to edit nfd-worker daemonset to
tolerate the taints to be created. Otherwise, already running pods that do not
tolerate the taint are evicted immediately from the node including the nfd-worker
pod.

Example NodeFeatureRule with custom taints:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-sample-rule-object
spec:
  rules:
    - name: "my sample taint rule"
      taints:
        - effect: PreferNoSchedule
          key: "feature.node.kubernetes.io/special-node"
          value: "true"
        - effect: NoExecute
          key: "feature.node.kubernetes.io/dedicated-node"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            dummy: {op: Exists}
        - feature: kernel.config
          matchExpressions:
            X86: {op: In, value: ["y"]}
```

In this example, if the `my sample taint rule` rule is matched, `feature.node.kubernetes.io/pci-0300_1d0f.present=true:NoExecute`
and `feature.node.kubernetes.io/cpu-cpuid.ADX:NoExecute` taints are set on the node.

There are some limitations to the namespace part (i.e. prefix/) of the taint
key:

- `kubernetes.io/` and its sub-namespaces (like `sub.ns.kubernetes.io/`) cannot
  generally be used
- the only exception is `feature.node.kubernetes.io/` and its sub-namespaces
  (like `sub.ns.feature.node.kubernetes.io`)
- unprefixed keys (like `foo`) keys are disallowed

## Local feature source

NFD-Worker has a special feature source named `local` which is an integration
point for external feature detectors. It provides a mechanism for pluggable
extensions, allowing the creation of new user-specific features and even
overriding built-in labels.

The `local` feature source has two methods for detecting features, hooks and
feature files. The features discovered by the `local` source can further be
used in label rules specified in
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects and the
[`custom`](#custom-feature-source) feature source.

**NOTE:** Be careful when creating and/or updating hook or feature files while
NFD is running. In order to avoid race conditions you should write into a
temporary file (outside the `source.d` and `features.d` directories), and,
atomically create/update the original file by doing a filesystem move
operation.

### A hook example

Consider a shell script
`/etc/kubernetes/node-feature-discovery/source.d/my-hook.sh` having the
following stdout output, or alternatively, a plaintext file
`/etc/kubernetes/node-feature-discovery/features.d/my-features` having the
following contents:

```plaintext
my-feature.1
my-feature.2=myvalue
my.namespace/my-feature.3=456
```

This will translate into the following node labels:

```yaml
feature.node.kubernetes.io/my-feature.1: "true"
feature.node.kubernetes.io/my-feature.2: "myvalue"
my.namespace/my-feature.3: "456"
```

### Hooks

**DEPRECATED** The `local` source executes hooks found in
`/etc/kubernetes/node-feature-discovery/source.d/`. The hook files must be
executable and they are supposed to print all discovered features in `stdout`.
Since NFD v0.13 the default container image only supports statically linked ELF
binaries.

`stderr` output of hooks is propagated to NFD log so it can be used for
debugging and logging.

NFD tries to execute any regular files found from the hooks directory.
Any additional data files the hook might need (e.g. a configuration file)
should be placed in a separate directory in order to avoid NFD unnecessarily
trying to execute them. A subdirectory under the hooks directory can be used,
for example `/etc/kubernetes/node-feature-discovery/source.d/conf/`.

**NOTE:** Hooks are being DEPRECATED and will be removed in a future release.
Starting from release v0.14 hooks are disabled by default and can be enabled
via `sources.local.hooksEnabled` field in the worker configuration.

```yaml
sources:
  local:
    hooksEnabled: true  # true by default at this point
```

**NOTE:** NFD will blindly run any executables placed/mounted in the hooks
directory. It is the user's responsibility to review the hooks for e.g.
possible security implications.

**NOTE:** The [full](../deployment/image-variants.md#full) image variant
provides backwards-compatibility with older NFD versions by including a more
expanded environment, supporting bash and perl runtimes.

### Feature files

The `local` source reads files found in
`/etc/kubernetes/node-feature-discovery/features.d/`.

### Input format

The hook stdout and feature files are expected to contain features in simple
key-value pairs, separated by newlines:

```plaintext
<name>[=<value>]
```

The label value defaults to `true`, if not specified.

Label namespace may be specified with `<namespace>/<name>[=<value>]`.

### Mounts

The standard NFD deployments contain `hostPath` mounts for
`/etc/kubernetes/node-feature-discovery/source.d/` and
`/etc/kubernetes/node-feature-discovery/features.d/`, making these directories
from the host available inside the nfd-worker container.

#### Injecting labels from other pods

One use case for the hooks and/or feature files is detecting features in other
Pods outside NFD, e.g. in Kubernetes device plugins. By using the same
`hostPath` mounts for `/etc/kubernetes/node-feature-discovery/source.d/` and
`/etc/kubernetes/node-feature-discovery/features.d/` in the side-car (e.g.
device plugin) creates a shared area for deploying hooks and feature files to
NFD. NFD will periodically scan the directories and run any hooks and read any
feature files it finds.

## Custom feature source

The `custom` feature source in nfd-worker provides a rule-based mechanism for
label creation, similar to the
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects. The difference is
that the rules are specified in the worker configuration instead of a
Kubernetes API object.

See [worker configuration](nfd-worker.md#worker-configuration)
for instructions how to set-up and manage the worker configuration.

### An example custom feature source configuration

Consider the following referential configuration for nfd-worker:

```yaml
core:
  labelSources: ["custom"]
sources:
  custom:
    - name: "my sample rule"
      labels:
        "my-sample-feature": "true"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            dummy: {op: Exists}
        - feature: kernel.config
          matchExpressions:
            X86: {op: In, value: ["y"]}
```

It specifies one rule which creates node label
`feature.node.kubenernetes.io/my-sample-feature=true` if both of the following
conditions are true (`matchFeatures` implements a logical AND over the
matchers):

- The `dummy` network driver module has been loaded
- X86 option in kernel config is set to `=y`

In addition, the configuration only enables the `custom` source, disabling all
built-in labels.

Now, on X86 platforms the feature label appears after doing `modprobe dummy` on
a system and correspondingly the label is removed after `rmmod dummy`. Note a
re-labeling delay up to the sleep-interval of nfd-worker (1 minute by default).

### Additional configuration directory

In addition to the rules defined in the nfd-worker configuration file, the
`custom` feature source can read more configuration files located in the
`/etc/kubernetes/node-feature-discovery/custom.d/` directory. This makes more
dynamic and flexible configuration easier.

As an example, consider having file
`/etc/kubernetes/node-feature-discovery/custom.d/my-rule.yaml` with the
following content:

```yaml
- name: "my e1000 rule"
  labels:
    "e1000.present": "true"
  matchFeatures:
    - feature: kernel.loadedmodule
      matchExpressions:
        e1000: {op: Exists}
```

This simple rule will create `feature.node.kubenernetes.io/e1000.present=true`
label if the `e1000` kernel module has been loaded.

The
[`samples/custom-rules`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/samples/custom-rules)
kustomize overlay sample contains an example for deploying a custom rule from a
ConfigMap.

## Node labels

Feature labels have the following format:

```plaintext
<namespace>/<name> = <value>
```

The namespace part (i.e. prefix) of the labels is controlled by nfd:

- All built-in labels use `feature.node.kubernetes.io`. This is also
  the default for user defined features that don't specify any namespace.
- Namespaces may be excluded with the
  [`-deny-label-ns`](../reference/master-commandline-reference.md#-deny-label-ns)
  command line flag of nfd-master
  - To allow specific namespaces that were denied, you can use
    [`-extra-label-ns`](../reference/master-commandline-reference.md#-extra-label-ns)
    command line flag of nfd-master.
    e.g: `nfd-master -deny-label-ns="*" -extra-label-ns=example.com`

## Label rule format

This section describes the rule format used  in
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects and in the
configuration of the [`custom`](#custom-feature-source) feature source.

It is based on a generic feature matcher that covers all features discovered by
nfd-worker. The rules rely on a unified data model of the available features
and a generic expression-based format. Features that can be used in the rules
are described in detail in [available features](#available-features) below.

Take this rule as a referential example:

```yaml
    - name: "my feature rule"
      labels:
        "my-special-feature": "my-value"
      matchFeatures:
        - feature: cpu.cpuid
          matchExpressions:
            AVX512F: {op: Exists}
        - feature: kernel.version
          matchExpressions:
            major: {op: In, value: ["5"]}
            minor: {op: Gt, value: ["1"]}
        - feature: pci.device
          matchExpressions:
            vendor: {op: In, value: ["8086"]}
            class: {op: In, value: ["0200"]}
```

This will yield `feature.node.kubenernetes.io/my-special-feature=my-value` node
label if all of these are true (`matchFeatures` implements a logical AND over
the matchers):

- the CPU has AVX512F capability
- kernel version is 5.2 or later (must be v5.x)
- an Intel network controller is present

### Fields

#### Name

The `.name` field is required and used as an identifier of the rule.

#### Labels

The `.labels` is a map of the node labels to create if the rule matches.

Take this rule as a referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-sample-rule-object
spec:
  rules:
    - name: "my dynamic label value rule"
      labels:
        linux-lsm-enabled: "@kernel.config.LSM"
        custom-label: "customlabel"
```

Label `linux-lsm-enabled` uses the `@` notation for dynamic values.
The value of the label will be the value of the attribute `LSM`
of the feature `kernel.config`.

The `@<feature-name>.<element-name>` format can be used to inject values of
detected features to the label. See
[available features](#available-features) for possible values to use.

This will yield into the following node label:

```yaml
  labels:
    ...
    feature.node.kubernetes.io/linux-lsm-enabled: apparmor
    feature.node.kubernetes.io/custom-label: "customlabel"
```

#### Labels template

The `.labelsTemplate` field specifies a text template for dynamically creating
labels based on the matched features. See [templating](#templating) for
details.

**NOTE** The `labels` field has priority over `labelsTemplate`, i.e.
labels specified in the `labels` field will override anything
originating from `labelsTemplate`.

#### Taints

*taints* is a list of taint entries and each entry can have `key`, `value` and `effect`,
where the `value` is optional. Effect could be `NoSchedule`, `PreferNoSchedule`
or `NoExecute`. To learn more about the meaning of these effects, check out k8s [documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).

**NOTE** taints field is not available for the custom rules of nfd-worker and only
for NodeFeatureRule objects.

#### Vars

The `.vars` field is a map of values (key-value pairs) to store for subsequent
rules to use. In other words, these are variables that are not advertised as
node labels. See [backreferences](#backreferences) for more details on the
usage of vars.

#### Extended resources

The `.extendedResources` field is a list of extended resources to advertise.
See [extended resources](#extended-resources) for more details.

Take this rule as a referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-extended-resource-rule
spec:
  rules:
    - name: "my extended resource rule"
      extendedResources:
        vendor.io/dynamic: "@kernel.version.major"
        vendor.io/static: "123"
      matchFeatures:
        - feature: kernel.version
          matchExpressions:
            major: {op: Exists}
```

The extended resource `vendor.io/dynamic` is defined in the form `@feature.attribute`.
The value of the extended resource will be the value of the attribute `major`
of the feature `kernel.version`.

The `@<feature-name>.<element-name>` format can be used to inject values of
detected features to the extended resource. See
[available features](#available-features) for possible values to use. Note that
the value must be eligible as a
Kubernetes resource quantity.

This will yield into the following node status:

```yaml
  allocatable:
    ...
    vendor.io/dynamic: "5"
    vendor.io/static: "123"
    ...
  capacity:
    ...
    vendor.io/dynamic: "5"
    vendor.io/static: "123"
    ...
```

There are some limitations to the namespace part (i.e. prefix)/ of the Extended
Resources names:

- `kubernetes.io/` and its sub-namespaces (like `sub.ns.kubernetes.io/`) cannot
  generally be used
- the only exception is `feature.node.kubernetes.io/` and its sub-namespaces
  (like `sub.ns.feature.node.kubernetes.io`)
- unprefixed names will get prefixed with `feature.node.kubernetes.io/`
  automatically (e.g. `foo` becomes `feature.node.kubernetes.io/foo`)

#### Vars template

The `.varsTemplate` field specifies a text template for dynamically creating
vars based on the matched features. See [templating](#templating) for details
on using templates and [backreferences](#backreferences) for more details on
the usage of vars.

**NOTE** The `vars` field has priority over `varsTemplate`, i.e.
vars specified in the `vars` field will override anything originating from
`varsTemplate`.

#### MatchFeatures

The `.matchFeatures` field specifies a feature matcher, consisting of a list of
feature matcher terms. It implements a logical AND over the terms i.e. all
of them must match in order for the rule to trigger.

```yaml
      matchFeatures:
        - feature: <feature-name>
          matchExpressions:
            <key>:
              op: <op>
              value:
                - <value-1>
                - ...
```

The `.matchFeatures[].feature` field specifies the feature against which to
match.

The `.matchFeatures[].matchExpressions` field specifies a map of expressions
which to evaluate against the elements of the feature.

In each MatchExpression `op` specifies the operator to apply. Valid values are
described below.

| Operator        | Number of values | Matches when
| --------------- | ---------------- | -----------
|  `In`           | 1 or greater | Input is equal to one of the values
|  `NotIn`        | 1 or greater | Input is not equal to any of the values
|  `InRegexp`     | 1 or greater | Values of the MatchExpression are treated as regexps and input matches one or more of them
|  `Exists`       | 0            | The key exists
|  `DoesNotExist` | 0            | The key does not exists
|  `Gt`           | 1            | Input is greater than the value. Both the input and value must be integer numbers.
|  `Lt`           | 1            | Input is less than the value. Both the input and value must be integer numbers.
|  `GtLt`         | 2            | Input is between two values. Both the input and value must be integer numbers.
|  `IsTrue`       | 0            | Input is equal to "true"
|  `IsFalse`      | 0            | Input is equal "false"

The `value` field of MatchExpression is a list of string arguments to the
operator.

The behavior of MatchExpression depends on the [feature type](#feature-types):
for *flag* and *attribute* features the MatchExpression operates on the feature
element whose name matches the `<key>`. However, for *instance* features all
MatchExpressions are evaluated against the attributes of each instance
separately.

#### MatchAny

The `.matchAny` field is a list of of [`matchFeatures`](#matchfeatures)
matchers. A logical OR is applied over the matchers, i.e. at least one of them
must match in order for the rule to trigger.

Consider the following example:

```yaml
      matchAny:
        - matchFeatures:
            - feature: kernel.loadedmodule
              matchExpressions:
                kmod-1: {op: Exists}
            - feature: pci.device
              matchExpressions:
                vendor: {op: In, value: ["0eee"]}
                class: {op: In, value: ["0200"]}
        - matchFeatures:
            - feature: kernel.loadedmodule
              matchExpressions:
                kmod-2: {op: Exists}
            - feature: pci.device
              matchExpressions:
                vendor: {op: In, value: ["0fff"]}
                class: {op: In, value: ["0200"]}
```

This matches if kernel module kmod-1 is loaded and a network controller from
vendor 0eee is present, OR, if kernel module kmod-2 has been loaded and a
network controller from vendor 0fff is present (OR both of these conditions are
true).

### Available features

The following features are available for matching:

| Feature          | [Feature type](#feature-types) | Elements | Value type | Description
| ---------------- | ------------ | -------- | ---------- | -----------
| **`cpu.cpuid`**  | flag         |          |            | Supported CPU capabilities
|                  |              | **`<cpuid-flag>`** |  | CPUID flag is present
| **`cpu.cstate`** | attribute    |          |            | Status of cstates in the intel_idle cpuidle driver
|                  |              | **`enabled`** | bool  | 'true' if cstates are set, otherwise 'false'. Does not exist of intel_idle driver is not active.
| **`cpu.model`**  | attribute    |          |            | CPU model related attributes
|                  |              | **`family`** | int    | CPU family
|                  |              | **`vendor_id`** | string | CPU vendor ID
|                  |              | **`id`** | int        | CPU model ID
| **`cpu.pstate`** | attribute    |          |            | State of the Intel pstate driver. Does not exist if the driver is not enabled.
|                  |              | **`status`** | string | Status of the driver, possible values are 'active' and 'passive'
|                  |              | **`turbo`**  | bool   | 'true' if turbo frequencies are enabled, otherwise 'false'
|                  |              | **`scaling`** | string | Active scaling_governor, possible values are 'powersave' or 'performance'.
| **`cpu.rdt`**    | attribute    |          |            | Intel RDT capabilities supported by the system
|                  |              | **`<rdt-flag>`** |    | RDT capability is supported, see [RDT flags](#intel-rdt-flags) for details
|                  |              | **`RDTL3CA_NUM_CLOSID`** | int  | The number or available CLOSID (Class of service ID) for Intel L3 Cache Allocation Technology
| **`cpu.security`** | attribute  |          |            | Features related to security and trusted execution environments
|                  |              | **`sgx.enabled`** | bool | `true` if Intel SGX (Software Guard Extensions) has been enabled, otherwise does not exist
|                  |              | **`sgx.epc`** | int | The total amount Intel SGX Encrypted Page Cache memory in bytes. It's only present if `sgx.enabled` is `true`.
|                  |              | **`se.enabled`** | bool  | `true` if IBM Secure Execution for Linux is available and has been enabled, otherwise does not exist
|                  |              | **`tdx.enabled`** | bool | `true` if Intel TDX (Trusted Domain Extensions) is available on the host and has been enabled, otherwise does not exist
|                  |              | **`tdx.total_keys`** | int | The total amount of keys an Intel TDX (Trusted Domain Extensions) host can provide.  It's only present if `tdx.enabled` is `true`.
|                  |              | **`tdx.protected`** | bool | `true` if a guest VM was started using Intel TDX (Trusted Domain Extensions), otherwise does not exist.
|                  |              | **`sev.enabled`** | bool | `true` if AMD SEV (Secure Encrypted Virtualization) is available on the host and has been enabled, otherwise does not exist
|                  |              | **`sev.es.enabled`** | bool | `true` if AMD SEV-ES (Encrypted State supported) is available on the host and has been enabled, otherwise does not exist
|                  |              | **`sev.snp.enabled`** | bool | `true` if AMD SEV-SNP (Secure Nested Paging supported) is available on the host and has been enabled, otherwise does not exist
| **`cpu.sgx`**    | attribute    |          |            | **DEPRECATED**: replaced by **`cpu.security`** feature
|                  |              | **`enabled`** | bool  | **DEPRECATED**: use **`sgx.enabled`** from **`cpu.security`** instead
| **`cpu.sst`**    | attribute    |          |            | Intel SST (Speed Select Technology) capabilities
|                  |              | **`bf.enabled`** | bool | `true` if Intel SST-BF (Intel Speed Select Technology - Base frequency) has been enabled, otherwise does not exist
| **`cpu.se`**     | attribute    |          |            | **DEPRECATED**: replaced by **`cpu.security`** feature
|                  |              | **`enabled`** | bool  | **DEPRECATED**: use **`se.enabled`** from **`cpu.security`** instead
| **`cpu.topology`** | attribute  |          |            | CPU topology related features
| | |          **`hardware_multithreading`** | bool       | Hardware multithreading, such as Intel HTT, is enabled
| **`cpu.coprocessor`** | attribute |        |            | CPU Coprocessor related features
| | |          **`nx_gzip`**                 | bool       | Nest Accelerator GZIP support is enabled
| **`kernel.config`** | attribute |          |            | Kernel configuration options
|                  |              | **`<config-flag>`** | string | Value of the kconfig option
| **`kernel.loadedmodule`** | flag |         |            | Kernel modules loaded on the node as reported by `/proc/modules`
| **`kernel.enabledmodule`** | flag |        |            | Kernel modules loaded on the node and available as built-ins as reported by `modules.builtin`
|                  |              | **`mod-name`** |      | Kernel module `<mod-name>` is loaded
| **`kernel.selinux`** | attribute |         |            | Kernel SELinux related features
|                  |              | **`enabled`** | bool  | `true` if SELinux has been enabled and is in enforcing mode, otherwise `false`
| **`kernel.version`** | attribute |          |           | Kernel version information
|                  |              | **`full`** | string   | Full kernel version (e.g. ‘4.5.6-7-g123abcde')
|                  |              | **`major`** | int     | First component of the kernel version (e.g. ‘4')
|                  |              | **`minor`** | int     | Second component of the kernel version (e.g. ‘5')
|                  |              | **`revision`** | int  | Third component of the kernel version (e.g. ‘6')
| **`local.label`** | attribute   |           |           | Features from hooks and feature files, i.e. labels from the [*local* feature source](#local-feature-source)
|                  |              | **`<label-name>`** | string | Label `<label-name>` created by the local feature source, value equals the value of the label
| **`memory.nv`**  | instance     |          |            | NVDIMM devices present in the system
|                  |              | **`<sysfs-attribute>`** | string | Value of the sysfs device attribute, available attributes: `devtype`, `mode`
| **`memory.numa`**  | attribute  |          |            | NUMA nodes
|                  |              | **`is_numa`** | bool  | `true` if NUMA architecture, `false` otherwise
|                  |              | **`node_count`** | int | Number of NUMA nodes
| **`network.device`** | instance |          |            | Physical (non-virtual) network interfaces present in the system
|                  |              | **`name`** | string   | Name of the network interface
|                  |              | **`<sysfs-attribute>`** | string | Sysfs network interface attribute, available attributes: `operstate`, `speed`, `sriov_numvfs`, `sriov_totalvfs`
| **`pci.device`** | instance     |          |            | PCI devices present in the system
|                  |              | **`<sysfs-attribute>`** | string | Value of the sysfs device attribute, available attributes: `class`, `vendor`, `device`, `subsystem_vendor`, `subsystem_device`, `sriov_totalvfs`, `iommu_group/type`, `iommu/intel-iommu/version`
| **`storage.device`** | instance |          |            | Block storage devices present in the system
|                  |              | **`name`** | string   | Name of the block device
|                  |              | **`<sysfs-attribute>`** | string | Sysfs network interface attribute, available attributes: `dax`, `rotational`, `nr_zones`, `zoned`
| **`system.osrelease`** | attribute |          |            | System identification data from `/etc/os-release`
|                  |              | **`<parameter>`** | string | One parameter from `/etc/os-release`
| **`system.name`** | attribute   |          |            | System name information
|                  |              | **`nodename`** | string | Name of the kubernetes node object
| **`usb.device`** | instance     |          |            | USB devices present in the system
|                  |              | **`<sysfs-attribute>`** | string | Value of the sysfs device attribute, available attributes: `class`, `vendor`, `device`, `serial`
| **`rule.matched`** | attribute  |          |            | Previously matched rules
|                  |              | **`<label-or-var>`** | string | Label or var from a preceding rule that matched

#### Intel RDT flags

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| RDTMON    | Intel RDT Monitoring Technology
| RDTCMT    | Intel Cache Monitoring (CMT)
| RDTMBM    | Intel Memory Bandwidth Monitoring (MBM)
| RDTL3CA   | Intel L3 Cache Allocation Technology
| RDTl2CA   | Intel L2 Cache Allocation Technology
| RDTMBA    | Intel Memory Bandwidth Allocation (MBA) Technology

### Templating

Rules support template-based creation of labels and vars with the
`.labelsTemplate` and `.varsTemplate` fields. These makes it possible to
dynamically generate labels and vars based on the features that matched.

The template must expand into a simple format with `<key>=<value>` pairs
separated by newline.

Consider the following example:
<!-- {% raw %} -->

```yaml
    labelsTemplate: |
      {{ range .pci.device }}vendor-{{ .class }}-{{ .device }}.present=true
      {{ end }}
    matchFeatures:
      - feature: pci.device
        matchExpressions:
          class: {op: InRegexp, value: ["^02"]}
          vendor: ["0fff"]
```

<!-- {% endraw %} -->
The rule above will create individual labels
`feature.node.kubernetes.io/vendor-<class-id>-<device-id>.present=true` for
each network controller device (device class starting with 02) from vendor
0ffff.

All the matched features of each feature matcher term under `matchFeatures`
fields are available for the template engine. Matched features can be
referenced with `{%raw%}{{ .<feature-name> }}{%endraw%}` in the template, and
the available data could be described in yaml as follows:

```yaml
.
  <key-feature>:
    - Name: <matched-key>
    - ...

  <value-feature>:
    - Name: <matched-key>
      Value: <matched-value>
    - ...

  <instance-feature>:
    - <attribute-1-name>: <attribute-1-value>
      <attribute-2-name>: <attribute-2-value>
      ...
    - ...
```

That is, the per-feature data is a list of objects whose data fields depend on
the type of the feature:

- for *flag* features only 'Name' is available
- for *value* features 'Name' and 'Value' are available
- for *instance* features all attributes of the matched instance are available

A simple example of a template utilizing name and value from an *attribute*
feature:
<!-- {% raw %} -->

```yaml
    labelsTemplate: |
      {{ range .system.osrelease }}system-{{ .Name }}={{ .Value }}
      {{ end }}
    matchFeatures:
      - feature: system.osRelease
        matchExpressions:
          ID: {op: Exists}
          VERSION_ID.major: {op: Exists}
```

<!-- {% endraw %} -->
**NOTE** In case of matchAny is specified, the template is executed separately
against each individual `matchFeatures` field and the final set of labels will
be superset of all these separate template expansions. E.g. consider the
following:

```yaml
  - name: <name>
    labelsTemplate: <template>
    matchFeatures: <matcher#1>
    matchAny:
      - matchFeatures: <matcher#2>
      - matchFeatures: <matcher#3>
```

In the example above (assuming the overall result is a match) the template
would be executed on matcher#1 as well as on matcher#2 and/or matcher#3
(depending on whether both or only one of them match). All the labels from
these separate expansions would be created, i.e. the end result would be a
union of all the individual expansions.

Rule templates use the Golang [text/template](https://pkg.go.dev/text/template)
package and all its built-in functionality (e.g. pipelines and functions) can
be used. An example template taking use of the built-in `len` function,
advertising the number of PCI network controllers from a specific vendor:
<!-- {% raw %} -->

```yaml
    labelsTemplate: |
      num-intel-network-controllers={{ .pci.device | len }}
    matchFeatures:
      - feature: pci.device
        matchExpressions:
          vendor: {op: In, value: ["8086"]}
          class: {op: In, value: ["0200"]}

```

<!-- {% endraw %} -->
Imaginative template pipelines are possible, but care must be taken in order to
produce understandable and maintainable rule sets.

### Backreferences

Rules support referencing the output of preceding rules. This enables
sophisticated scenarios where multiple rules are combined together
to for more complex heuristics than a single rule can provide. The labels and
vars created by the execution of preceding rules are available as a special
`rule.matched` feature.

Consider the following configuration:

```yaml
  - name: "my kernel label rule"
    labels:
      kernel-feature: "true"
    matchFeatures:
      - feature: kernel.version
        matchExpressions:
          major: {op: Gt, value: ["4"]}

  - name: "my var rule"
    vars:
      nolabel-feature: "true"
    matchFeatures:
      - feature: cpu.cpuid
        matchExpressions:
          AVX512F: {op: Exists}
      - feature: pci.device
        matchExpressions:
          vendor: {op: In, value: ["0fff"]}
          device: {op: In, value: ["1234", "1235"]}

  - name: "my high level feature rule"
    labels:
      high-level-feature: "true"
    matchFeatures:
      - feature: rule.matched
        matchExpressions:
          kernel-feature: {op: IsTrue}
          nolabel-feature: {op: IsTrue}
```

The `feature.node.kubernetes.io/high-level-feature = true` label depends on the
two previous rules.

Note that when referencing rules across multiple
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects attention must be
paid to the ordering. `NodeFeatureRule` objects are processed in alphabetical
order (based on their `.metadata.name`).

### Examples

Some more configuration examples below.

Match certain CPUID features:

```yaml
  - name: "example cpuid rule"
    labels:
      my-special-cpu-feature: "true"
    matchFeatures:
      - feature: cpu.cpuid
        matchExpressions:
          AESNI: {op: Exists}
          AVX: {op: Exists}
```

Require a certain loaded kernel module and OS version:

```yaml
  - name: "my multi-feature rule"
    labels:
      my-special-multi-feature: "true"
    matchFeatures:
      - feature: kernel.loadedmodule
        matchExpressions:
          e1000: {op: Exists}
      - feature: system.osrelease
        matchExpressions:
          NAME: {op: InRegexp, values: ["^openSUSE"]}
          VERSION_ID.major: {op: Gt, values: ["14"]}
```

Require a loaded  kernel module and two specific PCI devices (both of which
must be present):

```yaml
  - name: "my multi-device rule"
    labels:
      my-multi-device-feature: "true"
    matchFeatures:
      - feature: kernel.loadedmodule
        matchExpressions:
          my-driver-module: {op: Exists}
      - pci.device:
          vendor: "0fff"
          device: "1234"
      - pci.device:
          vendor: "0fff"
          device: "abcd"
```

## Legacy custom rule syntax

**DEPRECATED**: use the new rule syntax instead.

The `custom` source supports the legacy `matchOn` rule syntax for
backwards-compatibility.

To aid in making the legacy rule syntax clearer, we define a general and a per
rule nomenclature, keeping things as consistent as possible.

### General nomenclature and definitions

```plaintext
Rule        :Represents a matching logic that is used to match on a feature.
Rule Input  :The input a Rule is provided. This determines how a Rule performs the match operation.
Matcher     :A composition of Rules, each Matcher may be composed of at most one instance of each Rule.
```

### Custom features format (using the nomenclature defined above)

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

The label is constructed by adding `custom-` prefix to the name field, label
value defaults to `true` if not specified in the rule spec:

```plaintext
feature.node.kubernetes.io/custom-<name> = <value>
```

### Matching process

Specifying Rules to match on a feature is done by providing a list of Matchers.
Each Matcher contains one or more Rules.

Logical _OR_ is performed between Matchers and logical _AND_ is performed
between Rules of a given Matcher.

### Rules

#### pciid rule

##### Nomenclature

```plaintext
Attribute   :A PCI attribute.
Element     :An identifier of the PCI attribute.
```

The PciId Rule allows matching the PCI devices in the system on the following
Attributes: `class`,`vendor` and `device`. A list of Elements is provided for
each Attribute.

##### Format

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

#### UsbId rule

##### Nomenclature

```plaintext
Attribute   :A USB attribute.
Element     :An identifier of the USB attribute.
```

The UsbId Rule allows matching the USB devices in the system on the following
Attributes: `class`,`vendor`, `device` and `serial`. A list of Elements is
provided for each Attribute.

##### Format

```yaml
usbId :
  class: [<class id>, ...]
  vendor: [<vendor id>,  ...]
  device: [<device id>, ...]
  serial: [<serial>, ...]
```

Matching is done by performing a logical _OR_ between Elements of an Attribute
and logical _AND_ between the specified Attributes for each USB device in the
system.  At least one Attribute must be specified. Missing attributes will not
partake in the matching process.

#### LoadedKMod rule

##### Nomenclature

```plaintext
Element     :A kernel module
```

The LoadedKMod Rule allows matching the loaded kernel modules in the system
against a provided list of Elements.

##### Format

```yaml
loadedKMod : [<kernel module>, ...]
```

Matching is done by performing logical _AND_ for each provided Element, i.e
the Rule will match if all provided Elements (kernel modules) are loaded in the
system.

#### CpuId rule

##### Nomenclature

```plaintext
Element     :A CPUID flag
```

The Rule allows matching the available CPUID flags in the system against a
provided list of Elements.

##### Format

```yaml
cpuId : [<CPUID flag string>, ...]
```

Matching is done by performing logical _AND_ for each provided Element, i.e the
Rule will match if all provided Elements (CPUID flag strings) are available in
the system.

#### Kconfig rule

##### Nomenclature

```plaintext
Element     :A Kconfig option
```

The Rule allows matching the kconfig options in the system against a provided
list of Elements.

##### Format

```yaml
kConfig: [<kernel config option ('y' or 'm') or '=<value>'>, ...]
```

Matching is done by performing logical _AND_ for each provided Element, i.e the
Rule will match if all provided Elements (kernel config options) are enabled
(`y` or `m`) or matching `=<value>` in the kernel.

#### Nodename rule

##### Nomenclature

```plaintext
Element     :A nodename regexp pattern
```

The Rule allows matching the node's name against a provided list of Elements.

##### Format

```yaml
nodename: [ <nodename regexp pattern>, ... ]
```

Matching is done by performing logical _OR_ for each provided Element, i.e the
Rule will match if one of the provided Elements (nodename regexp pattern)
matches the node's name.

### Legacy custom rule example

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
          serial: ["090129a"]
  - name: "my.combined.feature"
    matchOn:
      - loadedKMod : ["vendor_kmod1", "vendor_kmod2"]
        pciId:
          vendor: ["15b3"]
          device: ["1014", "1017"]
  - name: "vendor.feature.node.kubernetes.io/accumulated.feature"
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
  - name: "profile.node.kubernetes.io/my-datacenter"
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
  `vendor.feature.node.kubernetes.io/accumulated.feature=true` if
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
  `profile.node.kubernetes.io/my-datacenter=datacenter-1` if the node's name
  matches the `node-datacenter1-rack.*-server.*` pattern, e.g.
  `node-datacenter1-rack2-server42`
