---
title: "Topology Updater Cmdline Reference"
layout: default
sort: 5
---

# NFD-Topology-Updater Commandline Flags
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-topology-updater -help`.
In a docker container:

```bash
docker run {{ site.container_image }} \
nfd-topology-updater -help
```

### -h, -help

Print usage and exit.

### -version

Print version and exit.

### -config

The `-config` flag specifies the path of the nfd-topology-updater
configuration file to use.

Default: /etc/kubernetes/node-feature-discovery/nfd-topology-updater.conf

Example:

```bash
nfd-topology-updater -config=/opt/nfd/nfd-topology-updater.conf
```

### -no-publish

The `-no-publish` flag makes for a "dry-run" flag for nfd-topology-updater.
NFD-Topology-Updater runs resource hardware topology detection normally, but
[NodeResourceTopology](../usage/custom-resources.md#noderesourcetopology)
objects are not created or updated.

Default: *false*

Example:

```bash
nfd-topology-updater -no-publish
```

### -oneshot

The `-oneshot` flag causes nfd-topology-updater to exit after one pass of
resource hardware topology detection.

Default: *false*

Example:

```bash
nfd-topology-updater -oneshot -no-publish
```

### -kubeconfig

The `-kubeconfig` flag specifies the path to a kubeconfig file to use for
connecting to the Kubernetes API server. If not specified, the in-cluster
configuration is used.

Default: *empty*

Example:

```bash
nfd-topology-updater -kubeconfig=${HOME}/.kube/config
```

### -port

The `-port` flag specifies the port on which metrics and healthz endpoints are
served on.

Default: 8080

Example:

```bash
nfd-topology-updater -port=12345
```

### -sleep-interval

The `-sleep-interval` specifies the interval between resource hardware
topology re-examination (and CR updates). zero means no CR updates on interval basis.

Default: 60s

Example:

```bash
nfd-topology-updater -sleep-interval=1h
```

### -watch-namespace

The `-watch-namespace` specifies the namespace to ensure that resource
hardware topology examination only happens for the pods running in the
specified namespace. Pods that are not running in the specified namespace
are not considered during resource accounting. This is particularly useful
for testing/debugging purpose. A "*" value would mean that all the pods would
be considered during the accounting process.

Default: "*"

Example:

```bash
nfd-topology-updater -watch-namespace=rte
```

### -kubelet-config-uri

The `-kubelet-config-uri` specifies the path to the Kubelet's configuration.
Note that the URI can either be a local file (`file://` scheme, for example
`file:///var/lib/kubelet/config.yaml`; a bare path without `file://` is
rejected) or an HTTPS endpoint (`https://` scheme). Other schemes, including
`http://`, are rejected at startup.

Default:  `https://${NODE_ADDRESS}:10250/configz`

Example:

```bash
nfd-topology-updater -kubelet-config-uri=file:///var/lib/kubelet/config.yaml
```

### -api-auth-token-file

The `-api-auth-token-file` specifies the path to the api auth token file
which is used to retrieve Kubelet's configuration from Kubelet secure port,
only taking effect when `-kubelet-config-uri` is https.
Note that this token file must bind to a role that has the `get` capability to
`nodes/configz` resources (on 1.33+ nodes), or `get` `nodes/proxy` resources
(prior to 1.33). See [warnings](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/#get-nodes-proxy-warning)
about granting `nodes/proxy` permissions.

Default:  `/var/run/secrets/kubernetes.io/serviceaccount/token`

Example:

```bash
nfd-topology-updater -api-auth-token-file=/var/run/secrets/kubernetes.io/serviceaccount/token
```

### -podresources-socket

The `-podresources-socket` specifies the path to the Unix socket where kubelet
exports a gRPC service to enable discovery of in-use CPUs and devices, and to
provide metadata for them.

Default:  /host-var/lib/kubelet/pod-resources/kubelet.sock

Example:

```bash
nfd-topology-updater -podresources-socket=/var/lib/kubelet/pod-resources/kubelet.sock
```

### -pods-fingerprint

Enables compute and report the pod set fingerprint in the NRT.
A pod fingerprint is a compact representation of the "node state" regarding resources.

Default: `true`

Example:

```bash
nfd-topology-updater -pods-fingerprint=false
```

### -kubelet-state-dir

The `-kubelet-state-dir` specifies the path to the Kubelet state directory,
where state and checkpoint files are stored.
The files are mount as read-only and cannot be change by the updater.
Enabled by default.
Passing an empty string will disable the watching.

Default:  /host-var/lib/kubelet

Example:

```bash
nfd-topology-updater -kubelet-state-dir=/var/lib/kubelet
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
