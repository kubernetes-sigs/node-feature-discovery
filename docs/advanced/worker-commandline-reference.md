---
title: "Worker Cmdline Reference"
layout: default
sort: 3
---

# NFD-Worker Commandline Flags
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-worker --help`.
In a docker container:

```bash
docker run {{ site.container_image }} nfd-worker --help
```

### -h, --help

Print usage and exit.

### --version

Print version and exit.

### --config

The `--config` flag specifies the path of the nfd-worker configuration file to
use.

Default: /etc/kubernetes/node-feature-discovery/nfd-worker.conf

Example:

```bash
nfd-worker --config=/opt/nfd/worker.conf
```

### --options

The `--options` flag may be used to specify and override configuration file
options directly from the command line. The required format is the same as in
the config file i.e. JSON or YAML. Configuration options specified via this
flag will override those from the configuration file:

Default: *empty*

Example:

```bash
nfd-worker --options='{"sources":{"cpu":{"cpuid":{"attributeWhitelist":["AVX","AVX2"]}}}}'
```

### --server

The `--server` flag specifies the address of the nfd-master endpoint where to
connect to.

Default: localhost:8080

Example:

```bash
nfd-worker --server=nfd-master.nfd.svc.cluster.local:443
```

### --ca-file

The `--ca-file` is one of the three flags (together with `--cert-file` and
`--key-file`) controlling the mutual TLS authentication on the worker side.
This flag specifies the TLS root certificate that is used for verifying the
authenticity of nfd-master.

Default: *empty*

Note: Must be specified together with `--cert-file` and `--key-file`

Example:

```bash
nfd-worker --ca-file=/opt/nfd/ca.crt --cert-file=/opt/nfd/worker.crt --key-file=/opt/nfd/worker.key
```

### --cert-file

The `--cert-file` is one of the three flags (together with `--ca-file` and
`--key-file`) controlling mutual TLS authentication on the worker side. This
flag specifies the TLS certificate presented for authenticating outgoing
requests.

Default: *empty*

Note: Must be specified together with `--ca-file` and `--key-file`

Example:

```bash
nfd-workerr --cert-file=/opt/nfd/worker.crt --key-file=/opt/nfd/worker.key --ca-file=/opt/nfd/ca.crt
```

### --key-file

The `--key-file` is one of the three flags (together with `--ca-file` and
`--cert-file`) controlling the mutual TLS authentication on the worker side.
This flag specifies the private key corresponding the given certificate file
(`--cert-file`) that is used for authenticating outgoing requests.

Default: *empty*

Note: Must be specified together with `--cert-file` and `--ca-file`

Example:

```bash
nfd-worker --key-file=/opt/nfd/worker.key --cert-file=/opt/nfd/worker.crt --ca-file=/opt/nfd/ca.crt
```

### --server-name-override

The `--server-name-override` flag specifies the common name (CN) which to
expect from the nfd-master TLS certificate. This flag is mostly intended for
development and debugging purposes.

Default: *empty*

Example:

```bash
nfd-worker --server-name-override=localhost
```

### --sources

The `--sources` flag specifies a comma-separated list of enabled feature
sources. A special value `all` enables all feature sources.

Default: all

Example:

```bash
nfd-worker --sources=kernel,system,local
```

### --no-publish

The `--no-publish` flag disables all communication with the nfd-master, making
it a "dry-run" flag for nfd-worker. NFD-Worker runs feature detection normally,
but no labeling requests are sent to nfd-master.

Default: *false*

Example:

```bash
nfd-worker --no-publish
```

### --label-whitelist

The `--label-whitelist` specifies a regular expression for filtering feature
labels based on their name. Each label must match against the given reqular
expression in order to be published.

Note: The regular expression is only matches against the "basename" part of the
label, i.e. to the part of the name after '/'. The label namespace is omitted.

Default: *empty*

Example:

```bash
nfd-worker --label-whitelist='.*cpuid\.'
```

### --oneshot

The `--oneshot` flag causes nfd-worker to exit after one pass of feature
detection.

Default: *false*

Example:

```bash
nfd-worker --oneshot --no-publish
```

### --sleep-interval

The `--sleep-interval` specifies the interval between feature re-detection (and
node re-labeling). A non-positive value implies infinite sleep interval, i.e.
no re-detection or re-labeling is done.

Default: 60s

Example:

```bash
nfd-worker --sleep-interval=1h
```
