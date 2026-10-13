---
title: "Garbage Collector Cmdline Reference"
layout: default
sort: 7
---

# NFD-GC Commandline Flags
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-gc -help`.
In a docker container:

```bash
docker run {{ site.container_image }} \
nfd-gc -help
```

### -h, -help

Print usage and exit.

### -version

Print version and exit.

### -list-size

The pagination size to use when calling api-server to list nodefeatures.
Pagination is useful for controlling the load on api-server/etcd as the
nodefeature resources can be large.
A value of 0 will disable pagination.

Default: 200

Example:

```bash
nfd-gc -list-size=100
```

### -gc-interval

The `-gc-interval` specifies the interval between periodic garbage collector runs.

Default: 1h

Example:

```bash
nfd-gc -gc-interval=1h
```

### -kubeconfig

The `-kubeconfig` flag specifies the path to a kubeconfig file to use for
connecting to the Kubernetes API server. If not specified, the in-cluster
configuration is used.

Default: *empty*

Example:

```bash
nfd-gc -kubeconfig=${HOME}/.kube/config
```

### -port

The `-port` flag specifies the port on which metrics are served on.

Default: 8080

Example:

```bash
nfd-gc -port=12345
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
