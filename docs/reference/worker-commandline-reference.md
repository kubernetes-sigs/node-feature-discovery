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

### -no-owner-refs

The `-no-owner-refs` flag disables setting the owner references to Pod
of the NodeFeature object.

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

Logs at or above this threshold go to stderr.

Default: 2

#### -v

Number for the log level verbosity.

Default: 0

#### -vmodule

Comma-separated list of `pattern=N` settings for file-filtered logging.

Default: *empty*
