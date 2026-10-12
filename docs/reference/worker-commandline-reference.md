---
title: "Worker cmdline reference"
layout: default
sort: 2
---

# Commandline flags of nfd-worker
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-worker -help`.
In a docker container:

```bash
docker run {{ site.container_image }} nfd-worker -help
```

### -h, -help

Print usage and exit.

### -version

Print version and exit.

### -feature-gates

The `-feature-gates` flag is used to enable or disable non GA features.
The list of available feature gates can be found in the [feature gates documentation](feature-gates.md).

Example:

```bash
nfd-master -feature-gates NodeFeatureGroupAPI=true
```

### -config

The `-config` flag specifies the path of the nfd-worker configuration file to
use.

Default: /etc/kubernetes/node-feature-discovery/nfd-worker.conf

Example:

```bash
nfd-worker -config=/opt/nfd/worker.conf
```

### -options

The `-options` flag may be used to specify and override configuration file
options directly from the command line. The required format is the same as in
the config file i.e. JSON or YAML. Configuration options specified via this
flag will override those from the configuration file:

Default: *empty*

Example:

```bash
nfd-worker -options='{"sources":{"cpu":{"cpuid":{"attributeWhitelist":["AVX","AVX2"]}}}}'
```

### -kubeconfig

The `-kubeconfig` flag specifies the kubeconfig to use for connecting to the
Kubernetes API server. It is needed for manipulating
[NodeFeature](../usage/custom-resources.md#nodefeature) objects. An empty value
(which is also the default) implies in-cluster kubeconfig.

Default: *empty*

Example:

```bash
nfd-worker -kubeconfig ${HOME}/.kube/config
```

### -feature-sources

The `-feature-sources` flag specifies a comma-separated list of enabled feature
sources. A special value `all` enables all sources. Prefixing a source name
with `-` indicates that the source will be disabled instead - this is only
meaningful when used in conjunction with `all`. This command line flag allows
completely disabling the feature detection so that neither standard feature
labels are generated nor the raw feature data is available for custom rule
processing.  Consider using the `core.featureSources` config file option,
instead, allowing dynamic configurability.

> **NOTE:** This flag takes precedence over the `core.featureSources`
> configuration file option.

Default: all

Example:

```bash
nfd-worker -feature-sources=all,-pci
```

### -label-sources

The `-label-sources` flag specifies a comma-separated list of enabled label
sources. A special value `all` enables all sources. Prefixing a source name
with `-` indicates that the source will be disabled instead - this is only
meaningful when used in conjunction with `all`. Consider using the
`core.labelSources` config file option, instead, allowing dynamic
configurability.

> **NOTE:** This flag takes precedence over the `core.labelSources`
> configuration file option.

Default: all

Example:

```bash
nfd-worker -label-sources=kernel,system,local
```

### -no-publish-features

The `-no-publish-features` flag specifies a comma-separated list of feature keys
that are discovered but omitted from the published `NodeFeature` object, to
reduce its size. Each entry matches the `<source>.<feature>` key exactly, or as
a prefix when it ends with `*`. Discovery is not affected, so the features stay
available to label sources and inline custom rules. Consider using the
`core.noPublishFeatures` config file option, instead, allowing dynamic
configurability.

> **NOTE:** This flag takes precedence over the `core.noPublishFeatures`
> configuration file option. Only use it for features that no cluster-side
> `NodeFeatureRule` or `NodeFeatureGroup` consumes.

Default: *empty*

Example:

```bash
nfd-worker -no-publish-features=pci.device,usb.*
```

### -port

The `-port` flag specifies the port on which metrics and healthz endpoints are
served on.

Default: 8080

Example:

```bash
nfd-worker -port=12345
```

### -no-publish

The `-no-publish` flag disables all communication with the nfd-master and the
Kubernetes API server. It is effectively a "dry-run" flag for nfd-worker.
NFD-Worker runs feature detection normally, but no labeling requests are sent
to nfd-master and no NodeFeature objects are created or updated in the API
server.

> **NOTE:** This flag takes precedence over the
> [`core.noPublish`](worker-configuration-reference.md#corenopublish)
> configuration file option.

Default: *false*

Example:

```bash
nfd-worker -no-publish
```

### -owner-refs

The `-owner-refs` flag selects the objects referenced as owners of the
NodeFeature object. It accepts a comma-separated list containing `node`, `pod`
and/or `ds`. An explicitly empty value disables owner references.

The owners have different lifecycle semantics:

- `node` ties the NodeFeature to the UID of the current Node.
- `pod` ties it to the UID of the nfd-worker Pod.
- `ds` ties it to the DaemonSet that owns the nfd-worker Pod.

When multiple owners are configured, Kubernetes keeps the NodeFeature while at
least one owner still exists. For example, `node,ds` does not cause native
garbage collection on a Node rebuild while the DaemonSet exists.

Selecting `node` requires the worker service account to have `get` permission
for Nodes. The Helm chart creates this permission when `worker.ownerRefs`
contains `node`. Kustomize installations must include the
[`worker-node-rbac`](https://github.com/kubernetes-sigs/node-feature-discovery/tree/{{site.release}}/deployment/components/worker-node-rbac)
component. The NFD garbage collector may still explicitly delete stale
NodeFeature objects independently of native owner-reference garbage collection.

> **NOTE:** This flag takes precedence over the
> [`core.ownerRefs`](worker-configuration-reference.md#coreownerrefs)
> configuration file option.

Default: `pod,ds`

Example:

```bash
nfd-worker -owner-refs=node
```

### -no-owner-refs

The `-no-owner-refs` flag disables setting owner references on the NodeFeature
object. It takes precedence over `-owner-refs`. It is deprecated; use
`-owner-refs=` instead.

> **NOTE:** This flag takes precedence over the
> [`core.noOwnerRefs`](worker-configuration-reference.md#corenoownerrefs)
> configuration file option.

Default: *false*

Example:

```bash
nfd-worker -no-owner-refs
```

### -oneshot

The `-oneshot` flag causes nfd-worker to exit after one pass of feature
detection.

Default: *false*

Example:

```bash
nfd-worker -oneshot -no-publish
```

### Logging

The following logging-related flags are inherited from the
[klog](https://pkg.go.dev/k8s.io/klog/v2) package.

> **NOTE:** The logger setup can also be specified via the `core.klog`
> configuration file options. However, the command line flags take precedence
> over any corresponding config file options specified.

#### -add_dir_header

If true, adds the file directory to the header of the log messages.

Default: false

#### -alsologtostderr

Log to standard error as well as files.

Default: false

#### -log_backtrace_at

When logging hits line file:N, emit a stack trace.

Default: *empty*

#### -log_dir

If non-empty, write log files in this directory.

Default: *empty*

#### -log_file

If non-empty, use this log file.

Default: *empty*

#### -log_file_max_size

Defines the maximum size a log file can grow to. Unit is megabytes. If the
value is 0, the maximum file size is unlimited.

Default: 1800

#### -logtostderr

Log to standard error instead of files

Default: true

#### -skip_headers

If true, avoid header prefixes in the log messages.

Default: false

#### -skip_log_headers

If true, avoid headers when opening log files.

Default: false

#### -stderrthreshold

Logs at or above this threshold go to stderr when
`-legacy_stderr_threshold_behavior=false` (the default in NFD). Severity levels:
0=INFO, 1=WARNING, 2=ERROR, 3=FATAL.

Default: 0

#### -legacy_stderr_threshold_behavior

If true, -stderrthreshold is ignored when -logtostderr=true (legacy klog
behavior). NFD sets this to false so that -stderrthreshold is always honored.

Default: false

#### -v

Number for the log level verbosity.

Default: 0

#### -vmodule

Comma-separated list of `pattern=N` settings for file-filtered logging.

Default: *empty*
