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

### -prune

The `-prune` flag is a sub-command like option for cleaning up the cluster. It
causes nfd-master to remove all NFD related labels, annotations and extended
resources from all Node objects of the cluster and exit.

### -port

The `-port` flag specifies the TCP port that nfd-master listens for incoming requests.

Default: 8080

Example:

```bash
nfd-master -port=443
```

### -instance

The `-instance` flag makes it possible to run multiple NFD deployments in
parallel. In practice, it separates the node annotations between deployments so
that each of them can store metadata independently. The instance name must
start and end with an alphanumeric character and may only contain alphanumeric
characters, `-`, `_` or `.`.

Default: *empty*

Example:

```bash
nfd-master -instance=network
```

### -ca-file

The `-ca-file` is one of the three flags (together with `-cert-file` and
`-key-file`) controlling master-worker mutual TLS authentication on the
nfd-master side. This flag specifies the TLS root certificate that is used for
authenticating incoming connections. NFD-Worker side needs to have matching key
and cert files configured in order for the incoming requests to be accepted.

Default: *empty*

Note: Must be specified together with `-cert-file` and `-key-file`

Example:

```bash
nfd-master -ca-file=/opt/nfd/ca.crt -cert-file=/opt/nfd/master.crt -key-file=/opt/nfd/master.key
```

### -cert-file

The `-cert-file` is one of the three flags (together with `-ca-file` and
`-key-file`) controlling master-worker mutual TLS authentication on the
nfd-master side. This flag specifies the TLS certificate presented for
authenticating outgoing traffic towards nfd-worker.

Default: *empty*

Note: Must be specified together with `-ca-file` and `-key-file`

Example:

```bash
nfd-master -cert-file=/opt/nfd/master.crt -key-file=/opt/nfd/master.key -ca-file=/opt/nfd/ca.crt
```

### -key-file

The `-key-file` is one of the three flags (together with `-ca-file` and
`-cert-file`) controlling master-worker mutual TLS authentication on the
nfd-master side. This flag specifies the private key corresponding the given
certificate file (`-cert-file`) that is used for authenticating outgoing
traffic.

Default: *empty*

Note: Must be specified together with `-cert-file` and `-ca-file`

Example:

```bash
nfd-master -key-file=/opt/nfd/master.key -cert-file=/opt/nfd/master.crt -ca-file=/opt/nfd/ca.crt
```

### -verify-node-name

The `-verify-node-name` flag controls the NodeName based authorization of
incoming requests and only has effect when mTLS authentication has been enabled
(with `-ca-file`, `-cert-file` and `-key-file`). If enabled, the worker node
name of the incoming must match with the CN or a SAN in its TLS certificate. Thus,
workers are only able to label the node they are running on (or the node whose
certificate they present).

Node Name based authorization is disabled by default.

Default: *false*

Example:

```bash
nfd-master -verify-node-name -ca-file=/opt/nfd/ca.crt \
    -cert-file=/opt/nfd/master.crt -key-file=/opt/nfd/master.key
```

### -enable-nodefeature-api

The `-enable-nodefeature-api` flag enables the
[NodeFeature](../usage/custom-resources.md#nodefeature) CRD API for receiving
feature requests. This will also automatically disable the gRPC interface.

Default: false

Example:

```bash
nfd-master -enable-nodefeature-api
```

### -enable-leader-election

The `-enable-leader-election` flag enables leader election for NFD-Master.
It is advised to turn on this flag when running more than one instance of
NFD-Master.

This flag takes effect only when combined with `-enable-nodefeature-api` flag.

Default: false

```bash
nfd-master -enable-nodefeature-api -enable-leader-election
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

### -crd-controller

The `-crd-controller` flag specifies whether the NFD CRD API controller is
enabled or not. The controller is responsible for processing
[NodeFeature](../usage/custom-resources.md#nodefeature) and
[NodeFeatureRule](../usage/custom-resources.md#nodefeaturerule) objects.

Default: *true*

Example:

```bash
nfd-master -crd-controller=false
```

### -featurerules-controller

**DEPRECATED**: use [`-crd-controller`](#-crd-controller) instead.

### -label-whitelist

The `-label-whitelist` specifies a regular expression for filtering feature
labels based on their name. Each label must match against the given reqular
expression in order to be published.

Note: The regular expression is only matches against the "basename" part of the
label, i.e. to the part of the name after '/'. The label namespace is omitted.

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

The same namespace control and this flag applies Extended Resources (created
with `-resource-labels`), too.

Default: *empty*

Example:

```bash
nfd-master -extra-label-ns=vendor-1.com,vendor-2.io
```

### -deny-label-ns

The `-deny-label-ns` flag specifies a comma-separated list of excluded
label namespaces. By default, nfd-master allows creating labels in all
namespaces, excluding `kubernetes.io` namespace and its sub-namespaces
(i.e. `*.kubernetes.io`). However, you should note that
`kubernetes.io` and its sub-namespaces are always denied.
For example, `nfd-master -deny-label-ns=""` would still disallow
`kubernetes.io` and `*.kubernetes.io`.
This option can be used to exclude some vendors or application specific
namespaces.
Note that the namespaces `feature.node.kubernetes.io` and `profile.node.kubernetes.io`
and their sub-namespaces are always allowed and cannot be denied.

Default: *empty*

Example:

```bash
nfd-master -deny-label-ns=*.vendor.com,vendor-2.io
```

### -resource-labels

**DEPRECATED**: [NodeFeatureRule](../usage/custom-resources.md#nodefeaturerule)
should be used for managing extended resources in NFD.

The `-resource-labels` flag specifies a comma-separated list of features to be
advertised as extended resources instead of labels. Features that have integer
values can be published as Extended Resources by listing them in this flag.

Default: *empty*

Example:

```bash
nfd-master -resource-labels=vendor-1.com/feature-1,vendor-2.io/feature-2
```

### -config

The `-config` flag specifies the path of the nfd-master configuration file to
use.

Default: /etc/kubernetes/node-feature-discovery/nfd-master.conf

Example:

```bash
nfd-master -config=/opt/nfd/master.conf
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

It takes effect only when `-enable-nodefeature-api` has been set.

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

### -resync-period

The `-resync-period` flag specifies the NFD API controller resync period.
The resync means nfd-master replaying all NodeFeature and NodeFeatureRule objects,
thus effectively re-syncing all nodes in the cluster (i.e. ensuring labels, annotations,
extended resources and taints are in place).
Only has effect when the [NodeFeature](../usage/custom-resources.md#nodefeature)
CRD API has been enabled with [`-enable-nodefeature-api`](master-commandline-reference.md#-enable-nodefeature-api).

Default: 1 hour.

Example:

```bash
nfd-master -resync-period=2h
```
