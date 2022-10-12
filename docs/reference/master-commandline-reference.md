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

### -enable-taints

The `-enable-taints` flag enables/disables node tainting feature of NFD.

Default: *false*

Example:

```bash
nfd-master -enable-taints=true
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

The `-enable-nodefeature-api` flag enables the NodeFeature CRD API for
receiving feature requests. This will also automatically disable the gRPC
interface.

Default: false

Example:

```bash
nfd-master -enable-nodefeature-api
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

### -featurerules-controller

The `-featurerules-controller` flag controlers the processing of
NodeFeatureRule objects, effectively enabling/disabling labels from these
custom labeling rules.

Default: *true*

Example:

```bash
nfd-master -featurerules-controller=false
```

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
label namespaces. By default, nfd-master only allows creating labels in the
default `feature.node.kubernetes.io` and `profile.node.kubernetes.io` label
namespaces and their sub-namespaces (e.g. `vendor.feature.node.kubernetes.io`
and `sub.ns.profile.node.kubernetes.io`). This option can be used to allow
other vendor or application specific namespaces for custom labels from the
local and custom feature sources.

The same namespace control and this flag applies Extended Resources (created
with `-resource-labels`), too.

Default: *empty*

Example:

```bash
nfd-master -extra-label-ns=vendor-1.com,vendor-2.io
```

### -resource-labels

The `-resource-labels` flag specifies a comma-separated list of features to be
advertised as extended resources instead of labels. Features that have integer
values can be published as Extended Resources by listing them in this flag.

Default: *empty*

Example:

```bash
nfd-master -resource-labels=vendor-1.com/feature-1,vendor-2.io/feature-2
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
