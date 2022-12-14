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

### -server

The `-server` flag specifies the address of the nfd-master endpoint where to
connect to.

Default: localhost:8080

Example:

```bash
nfd-worker -server=nfd-master.nfd.svc.cluster.local:443
```

### -ca-file

The `-ca-file` is one of the three flags (together with `-cert-file` and
`-key-file`) controlling the mutual TLS authentication on the worker side.
This flag specifies the TLS root certificate that is used for verifying the
authenticity of nfd-master.

Default: *empty*

Note: Must be specified together with `-cert-file` and `-key-file`

Example:

```bash
nfd-worker -ca-file=/opt/nfd/ca.crt -cert-file=/opt/nfd/worker.crt -key-file=/opt/nfd/worker.key
```

### -cert-file

The `-cert-file` is one of the three flags (together with `-ca-file` and
`-key-file`) controlling mutual TLS authentication on the worker side. This
flag specifies the TLS certificate presented for authenticating outgoing
requests.

Default: *empty*

Note: Must be specified together with `-ca-file` and `-key-file`

Example:

```bash
nfd-workerr -cert-file=/opt/nfd/worker.crt -key-file=/opt/nfd/worker.key -ca-file=/opt/nfd/ca.crt
```

### -key-file

The `-key-file` is one of the three flags (together with `-ca-file` and
`-cert-file`) controlling the mutual TLS authentication on the worker side.
This flag specifies the private key corresponding the given certificate file
(`-cert-file`) that is used for authenticating outgoing requests.

Default: *empty*

Note: Must be specified together with `-cert-file` and `-ca-file`

Example:

```bash
nfd-worker -key-file=/opt/nfd/worker.key -cert-file=/opt/nfd/worker.crt -ca-file=/opt/nfd/ca.crt
```

### -kubeconfig

The `-kubeconfig` flag specifies the kubeconfig to use for connecting to the
Kubernetes API server. It is only needed for manipulating
[NodeFeature](../usage/custom-resources#nodefeature) objects, and thus the flag
only takes effect when
[`-enable-nodefeature-api`](#-enable-nodefeature-api)) is specified. An empty
value (which is also the default) implies in-cluster kubeconfig.

Default: *empty*

Example:

```bash
nfd-worker -kubeconfig ${HOME}/.kube/config
```

### -server-name-override

The `-server-name-override` flag specifies the common name (CN) which to
expect from the nfd-master TLS certificate. This flag is mostly intended for
development and debugging purposes.

Default: *empty*

Example:

```bash
nfd-worker -server-name-override=localhost
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

Note: This flag takes precedence over the `core.featureSources` configuration
file option.

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

Note: This flag takes precedence over the `core.labelSources` configuration
file option.

Default: all

Example:

```bash
nfd-worker -label-sources=kernel,system,local
```

### -enable-nodefeature-api

The `-enable-nodefeature-api` flag enables the experimental
[NodeFeature](../usage/custom-resources#nodefeature) CRD API
for communicating with nfd-master. This will also automatically disable the
gRPC communication to nfd-master. When enabled, nfd-worker will create per-node
NodeFeature objects the contain all discovered node features and the set of
feature labels to be created.

Default: false

Example:

```bash
nfd-worker -enable-nodefeature-api
```

### -no-publish

The `-no-publish` flag disables all communication with the nfd-master and the
Kubernetes API server. It is effectively a "dry-run" flag for nfd-worker.
NFD-Worker runs feature detection normally, but no labeling requests are sent
to nfd-master and no NodeFeature objects are created or updated in the API
server.

Note: This flag takes precedence over the
[`core.noPublish`](worker-configuration-reference#corenopublish)
configuration file option.

Default: *false*

Example:

```bash
nfd-worker -no-publish
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

Note: The logger setup can also be specified via the `core.klog` configuration
file options. However, the command line flags take precedence over any
corresponding config file options specified.

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
