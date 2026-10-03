---
title: "Master cmdline reference"
layout: default
sort: 1
---

# Commandline flags of nfd-master
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-master -help`.
In a docker container:

```bash
docker run {{ site.container_image }} nfd-master -help
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

### -prune

The `-prune` flag is a sub-command like option for cleaning up the cluster. It
causes nfd-master to remove all NFD related labels, annotations, extended
resources and taints from all Node objects of the cluster and exit. Pruning is
skipped if `-no-publish` (or the `noPublish` configuration option) is set.

> **NOTE:** Currently, if NFD taints are enabled, the first `nfd-master -prune`
> run may exit with `failed to patch the node` and leave the NFD taints on the
> node. Run it again. The Helm post-delete hook and the prune kustomize overlay
> run prune as a Job, which retries automatically.

### -port

The `-port` flag specifies the port on which metrics and healthz endpoints are
served on.

Default: 8080

Example:

```bash
nfd-master -port=12345
```

### -instance

The `-instance` flag makes it possible to run multiple NFD deployments in
parallel. In practice, it separates the node annotations between deployments so
that each of them can store metadata independently. The instance name must
start and end with an alphanumeric character and may only contain alphanumeric
characters, `-`, `_` or `.`.

The instance name is added as a prefix to the feature-labels,
feature-annotations and extended-resources annotations (for example
`network.nfd.node.kubernetes.io/feature-labels`). The
`nfd.node.kubernetes.io/taints` annotation does not get the prefix and is not
separated between deployments.

Default: *empty*

Example:

```bash
nfd-master -instance=network
```

### -enable-leader-election

The `-enable-leader-election` flag enables leader election for NFD-Master.
It is advised to turn on this flag when running more than one instance of
NFD-Master.

Default: false

```bash
nfd-master -enable-leader-election
```

### -enable-taints

The `-enable-taints` flag enables/disables node tainting feature of NFD.

Default: *false*

Example:

```bash
nfd-master -enable-taints=true
```

### -no-publish

The `-no-publish` flag disables updates to the Node objects in the Kubernetes
API server, making a "dry-run" flag for nfd-master. No Labels, Annotations or
ExtendedResources of nodes are updated.

Default: *false*

Example:

```bash
nfd-master -no-publish
```

### -label-whitelist

The `-label-whitelist` specifies a regular expression for filtering feature
labels based on their name. Each label must match against the given regular
expression or it will not be published.

> **NOTE:** The regular expression is only matches against the "basename" part
> of the label, i.e. to the part of the name after '/'. The label namespace is
> omitted.

Default: *empty*

Example:

```bash
nfd-master -label-whitelist='.*cpuid\.'
```

### -extra-label-ns

The `-extra-label-ns` flag specifies a comma-separated list of allowed feature
label namespaces. This option can be used to allow
other vendor or application specific namespaces for custom labels from the
local and custom feature sources, even though these labels were denied using
the `deny-label-ns` flag.

Default: *empty*

Example:

```bash
nfd-master -extra-label-ns=vendor-1.com,vendor-2.io
```

### -deny-label-ns

The `-deny-label-ns` flag specifies a comma-separated list of excluded
label namespaces. By default, nfd-master allows creating labels in all
namespaces, excluding `kubernetes.io` namespace and its sub-namespaces
(i.e. `*.kubernetes.io`). `kubernetes.io` and its sub-namespaces (except
`feature.node.kubernetes.io`, `profile.node.kubernetes.io` and their
sub-namespaces) stay denied whatever `-deny-label-ns` is set to. A specific
namespace can only be allowed by listing it in `-extra-label-ns`.
For example, `nfd-master -deny-label-ns=""` would still disallow
`kubernetes.io` and `*.kubernetes.io`.
This option can be used to exclude some vendors or application specific
namespaces.
Note that the `feature.node.kubernetes.io` and `profile.node.kubernetes.io`
namespaces and their sub-namespaces are allowed by default even though they
are under `kubernetes.io`. They can still be denied explicitly: for example
`-deny-label-ns=feature.node.kubernetes.io` removes all built-in feature
labels, and `-deny-label-ns=*.kubernetes.io` removes the labels in all of these
namespaces. A namespace listed in `-extra-label-ns` is allowed even when
`-deny-label-ns` denies it.

Default: *empty*

Example:

```bash
nfd-master -deny-label-ns=*.vendor.com,vendor-2.io
```

### -informer-page-size

The `-informer-page-size` flag is used to control pagination
during informer cache sync on nfd-master startup.
This is useful to control load on api-server/etcd as listing
NodeFeature objects can be expensive, especially in large clusters.

Default: 200

Example:

```bash
nfd-master -informer-page-size=20
```

### -config

The `-config` flag specifies the path of the nfd-master configuration file to
use.

Default: /etc/kubernetes/node-feature-discovery/nfd-master.conf

Example:

```bash
nfd-master -config=/opt/nfd/master.conf
```

### -kubeconfig

The `-kubeconfig` flag specifies the path to a kubeconfig file to use for
connecting to the Kubernetes API server. If not specified, the in-cluster
configuration is used.

Default: *empty*

Example:

```bash
nfd-master -kubeconfig=${HOME}/.kube/config
```

### -options

The `-options` flag may be used to specify and override configuration file
options directly from the command line. The required format is the same as in
the config file i.e. JSON or YAML. Configuration options specified via this
flag will override those from the configuration file:

Default: *empty*

Example:

```bash
nfd-master -options='{"noPublish": true}'
```

### -nfd-api-parallelism

The `-nfd-api-parallelism` flag can be used to specify the maximum
number of concurrent node updates.

Default: 10

Example:

```bash
nfd-master -nfd-api-parallelism=1
```

### Logging

The following logging-related flags are inherited from the
[klog](https://pkg.go.dev/k8s.io/klog/v2) package.

#### -add_dir_header

If true, adds the file directory to the header of the log messages.

Default: false

#### -alsologtostderr

Log to standard error as well as files.

Default: false

#### -alsologtostderrthreshold

Logs at or above this threshold go to stderr when -alsologtostderr=true (no
effect when -logtostderr=true).

Default: 0

#### -legacy_stderr_threshold_behavior

If true, stderrthreshold is ignored when logtostderr=true (legacy behavior). If
false, stderrthreshold is honored even when logtostderr=true.

Default: true

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

#### -one_output

If true, only write logs to their native severity level (vs also writing to
each lower severity level; no effect when -logtostderr=true).

Default: false

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

### -resync-period

The `-resync-period` flag specifies the NFD API controller resync period.
The resync means nfd-master replaying all NodeFeature and NodeFeatureRule objects,
thus effectively re-syncing all nodes in the cluster (i.e. ensuring labels, annotations,
extended resources and taints are in place).

Default: 1 hour.

Example:

```bash
nfd-master -resync-period=2h
```
