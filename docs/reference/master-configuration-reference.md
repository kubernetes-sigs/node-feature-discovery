---
title: "Master config reference"
layout: default
sort: 3
---

# Configuration file reference of nfd-master
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

See the
[sample configuration file](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/master-config/nfd-master.conf.example)
for a full example configuration.

## noPublish

`noPublish` option disables updates to the Node objects in the Kubernetes
API server, making a "dry-run" flag for nfd-master. No Labels, Annotations, Taints
or ExtendedResources of nodes are updated.

Default: `false`

Example:

```yaml
noPublish: true
```

## extraLabelNs
`extraLabelNs` specifies a list of allowed feature
label namespaces. This option can be used to allow
other vendor or application specific namespaces for custom labels from the
local and custom feature sources, even though these labels were denied using
the `denyLabelNs` parameter.

The same namespace control and this option applies to Extended Resources (created
with `resourceLabels`), too.

Default: *empty*

Example:

```yaml
extraLabelNs: ["added.ns.io","added.kubernets.io"]
```

## denyLabelNs
`denyLabelNs` specifies a list of excluded
label namespaces. By default, nfd-master allows creating labels in all
namespaces, excluding `kubernetes.io` namespace and its sub-namespaces
(i.e. `*.kubernetes.io`). However, you should note that
`kubernetes.io` and its sub-namespaces are always denied.
This option can be used to exclude some vendors or application specific
namespaces.

Default: *empty*

Example:

```yaml
denyLabelNs: ["denied.ns.io","denied.kubernetes.io"]
```

## autoDefaultNs

**DEPRECATED**: Will be removed in NFD v0.17. Use the
[DisableAutoPrefix](feature-gates.md#disableautoprefix) feature gate instead.

The `autoDefaultNs` option controls the automatic prefixing of names. When set
to true (the default in NFD version {{ site.version }}) nfd-master
automatically adds the default `feature.node.kubernetes.io/` prefix to
unprefixed labels, annotations and extended resources - this is also the
default behavior in NFD v0.15 and earlier. When the option is set to `false`,
no prefix will be prepended to unprefixed names, effectively causing them to be
filtered out (as NFD does not allow unprefixed names of labels, annotations or
extended resources).  The default will be changed to `false` in a future
release.

For example, with the `autoDefaultNs` set to `true`, a NodeFeatureRule with

```yaml
  labels:
    foo: bar
```

Will turn into `feature.node.kubernetes.io/foo=bar` node label. With
`autoDefaultNs` set to `false`, no prefix is added and the label will be
filtered out.

Note that taint keys are not affected by this option.

Default: `true`

Example:

```yaml
autoDefaultNs: false
```

## resourceLabels

**DEPRECATED**: [NodeFeatureRule](../usage/custom-resources.md#nodefeaturerule)
should be used for managing extended resources in NFD.

The `resourceLabels` option specifies a list of features to be
advertised as extended resources instead of labels. Features that have integer
values can be published as Extended Resources by listing them in this option.

Default: *empty*

Example:

```yaml
resourceLabels: ["vendor-1.com/feature-1","vendor-2.io/feature-2"]
```

## enableTaints
`enableTaints` enables/disables node tainting feature of NFD.

Default: *false*

Example:

```yaml
enableTaints: true
```

## labelWhiteList
`labelWhiteList` specifies a regular expression for filtering feature
labels based on their name. Each label must match against the given regular
expression or it will not be published.

> ** NOTE:** The regular expression is only matches against the "basename" part
> of the label, i.e. to the part of the name after '/'. The label namespace is
> omitted.

Default: *empty*

Example:

```yaml
labelWhiteList: "foo"
```

## resyncPeriod

The `resyncPeriod` option specifies the NFD API controller resync period.
The resync means nfd-master replaying all NodeFeature and NodeFeatureRule objects,
thus effectively re-syncing all nodes in the cluster (i.e. ensuring labels, annotations,
extended resources and taints are in place).

Default: 1 hour.

Example:

```yaml
resyncPeriod: 2h
```

## leaderElection

The `leaderElection` section exposes configuration to tweak leader election.

### leaderElection.leaseDuration

`leaderElection.leaseDuration` is the duration that non-leader candidates will
wait to force acquire leadership. This is measured against time of
last observed ack.

A client needs to wait a full LeaseDuration without observing a change to
the record before it can attempt to take over. When all clients are
shutdown and a new set of clients are started with different names against
the same leader record, they must wait the full LeaseDuration before
attempting to acquire the lease. Thus LeaseDuration should be as short as
possible (within your tolerance for clock skew rate) to avoid a possible
long waits in the scenario.

Default: 15 seconds.

Example:

```yaml
leaderElection:
  leaseDurtation: 15s
```

### leaderElection.renewDeadline

`leaderElection.renewDeadline` is the duration that the acting master will retry
refreshing leadership before giving up.

This value has to be lower than leaseDuration and greater than retryPeriod*1.2.

Default: 10 seconds.

Example:

```yaml
leaderElection:
  renewDeadline: 10s
```

### leaderElection.retryPeriod

`leaderElection.retryPeriod` is the duration the LeaderElector clients should wait
between tries of actions.

It has to be greater than 0.

Default: 2 seconds.

Example:

```yaml
leaderElection:
  retryPeriod: 2s
```

## nfdApiParallelism

The `nfdApiParallelism` option can be used to specify the maximum
number of concurrent node updates.

Default: 10

Example:

```yaml
nfdApiParallelism: 1
```

## klog

The following options specify the logger configuration. Most of which can be
dynamically adjusted at run-time.

> **NOTE:** The logger options can also be specified via command line flags
> which take precedence over any corresponding config file options.

### klog.addDirHeader

If true, adds the file directory to the header of the log messages.

Default: `false`

Run-time configurable: yes

### klog.alsologtostderr

Log to standard error as well as files.

Default: `false`

Run-time configurable: yes

### klog.logBacktraceAt

When logging hits line file:N, emit a stack trace.

Default: *empty*

Run-time configurable: yes

### klog.logDir

If non-empty, write log files in this directory.

Default: *empty*

Run-time configurable: no

### klog.logFile

If non-empty, use this log file.

Default: *empty*

Run-time configurable: no

### klog.logFileMaxSize

Defines the maximum size a log file can grow to. Unit is megabytes. If the
value is 0, the maximum file size is unlimited.

Default: `1800`

Run-time configurable: no

### klog.logtostderr

Log to standard error instead of files

Default: `true`

Run-time configurable: yes

### klog.skipHeaders

If true, avoid header prefixes in the log messages.

Default: `false`

Run-time configurable: yes

### klog.skipLogHeaders

If true, avoid headers when opening log files.

Default: `false`

Run-time configurable: no

### klog.stderrthreshold

Logs at or above this threshold go to stderr (default 2)

Run-time configurable: yes

### klog.v

Number for the log level verbosity.

Default: `0`

Run-time configurable: yes

### klog.vmodule

Comma-separated list of `pattern=N` settings for file-filtered logging.

Default: *empty*

Run-time configurable: yes
