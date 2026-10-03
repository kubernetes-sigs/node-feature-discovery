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

Default: *empty*

Example:

```yaml
extraLabelNs: ["added.ns.io","added.kubernets.io"]
```

## denyLabelNs
`denyLabelNs` specifies a list of excluded
label namespaces. By default, nfd-master allows creating labels in all
namespaces, excluding `kubernetes.io` and its sub-namespaces
(`*.kubernetes.io`), except `feature.node.kubernetes.io`,
`profile.node.kubernetes.io` and their sub-namespaces. `denyLabelNs` cannot
lift the `kubernetes.io` restriction; a specific namespace can be allowed with
`extraLabelNs`.
This option can be used to exclude some vendors or application specific
namespaces.

Default: *empty*

Example:

```yaml
denyLabelNs: ["denied.ns.io","denied.kubernetes.io"]
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
  leaseDuration: 15s
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

## informerPageSize

The `informerPageSize` option is used to control pagination
during informer cache sync on nfd-master startup.
This is useful to control load on api-server/etcd as listing
NodeFeature objects can be expensive, especially in large clusters.

Default: 200

Example:

```yaml
informerPageSize: 50
```

## klog

The following options specify the logger configuration. Changes take effect
when nfd-master is restarted (dynamic run-time reconfiguration was dropped in
NFD v0.17).

> **NOTE:** The logger options can also be specified via command line flags
> which take precedence over any corresponding config file options.

### klog.addDirHeader

If true, adds the file directory to the header of the log messages.

Default: `false`

### klog.alsologtostderr

Log to standard error as well as files.

Default: `false`

### klog.alsologtostderrthreshold

Logs at or above this threshold go to stderr when `alsologtostderr` is true (no
effect when `logtostderr` is true).

Default: `0`

### klog.legacyStderrThresholdBehavior

If true, `stderrthreshold` is ignored when `logtostderr` is true (legacy
behavior). If false, `stderrthreshold` is honored even when `logtostderr` is
true.

Default: `true`

### klog.logBacktraceAt

When logging hits line file:N, emit a stack trace.

Default: *empty*

### klog.logDir

If non-empty, write log files in this directory.

Default: *empty*

### klog.logFile

If non-empty, use this log file.

Default: *empty*

### klog.logFileMaxSize

Defines the maximum size a log file can grow to. Unit is megabytes. If the
value is 0, the maximum file size is unlimited.

Default: `1800`

### klog.logtostderr

Log to standard error instead of files

Default: `true`

### klog.oneOutput

If true, only write logs to their native severity level (vs also writing to
each lower severity level; no effect when `logtostderr` is true).

Default: `false`

### klog.skipHeaders

If true, avoid header prefixes in the log messages.

Default: `false`

### klog.skipLogHeaders

If true, avoid headers when opening log files.

Default: `false`

### klog.stderrthreshold

Logs at or above this threshold go to stderr (default 2)

### klog.v

Number for the log level verbosity.

Default: `0`

### klog.vmodule

Comma-separated list of `pattern=N` settings for file-filtered logging.

Default: *empty*

## restrictions (EXPERIMENTAL)

The following options specify the restrictions that can be applied by the
nfd-master on the deployed Custom Resources in the cluster.

### restrictions.nodeFeatureNamespaceSelector

The `nodeFeatureNamespaceSelector` option specifies the NodeFeatures namespaces
to watch, which can be selected by using `metav1.LabelSelector` as a type for
this option. An empty value selects all namespaces to be watched.

Default: *empty*

Example:

```yaml
restrictions:
  nodeFeatureNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: "node-feature-discovery"
    matchExpressions:
      - key: "kubernetes.io/metadata.name"
        operator: "In"
        values:
          - "node-feature-discovery"
```

### restrictions.disableLabels

The `disableLabels` option controls whether to allow creation of node labels
from NodeFeature and NodeFeatureRule CRs or not.

Default: false

Example:

```yaml
restrictions:
  disableLabels: true
```

### restrictions.disableExtendedResources

The `disableExtendedResources` option controls whether to allow creation of
node extended resources from NodeFeatureRule CR or not.

Default: false

Example:

```yaml
restrictions:
  disableExtendedResources: true
```

### restrictions.disableAnnotations

he `disableAnnotations` option controls whether to allow creation of node annotations
from NodeFeatureRule CR or not.

Default: false

Example:

```yaml
restrictions:
  disableAnnotations: true
```

### restrictions.allowOverwrite

The `allowOverwrite` option controls whether NFD is allowed to overwrite and
take over management of existing node labels, annotations, and extended resources.
Labels, annotations and extended resources created by NFD itself are not affected
(overwrite cannot be disabled). NFD tracks the labels, annotations and extended
resources that it manages with specific
[node annotations](../get-started/introduction.md#node-annotations).

Default: true

> **NOTE:** Currently `allowOverwrite: false` does not work as described above.
> Existing node labels and annotations that were not created by NFD are still
> overwritten, while value changes to labels and annotations that NFD already
> manages are no longer applied, and the
> `nfd.node.kubernetes.io/feature-labels` and
> `nfd.node.kubernetes.io/feature-annotations` tracking annotations stop being
> updated. Extended resources ignore this option and are always overwritten.
> Keep the default (`true`).

Example:

```yaml
restrictions:
  allowOverwrite: false
```

### restrictions.denyNodeFeatureLabels

The `denyNodeFeatureLabels` option specifies whether to deny labels from 3rd party
NodeFeature objects or not. NodeFeature objects created by nfd-worker are not affected.

Default: false

Example:

```yaml
restrictions:
  denyNodeFeatureLabels: true
```
