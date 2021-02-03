---
title: "Master Cmdline Reference"
layout: default
sort: 2
---

# NFD-Master Commandline Flags
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-master --help`.
In a docker container:

```bash
docker run {{ site.container_image }} nfd-master --help
```

### -h, --help

Print usage and exit.

### --version

Print version and exit.

### --prune

The `--prune` flag is a sub-command like option for cleaning up the cluster. It
causes nfd-master to remove all NFD related labels, annotations and extended
resources from all Node objects of the cluster and exit.

### --port

The `--port` flag specifies the TCP port that nfd-master listens for incoming requests.

Default: 8080

Example:

```bash
nfd-master --port=443
```

### --instance

The `--instance` flag makes it possible to run multiple NFD deployments in
parallel. In practice, it separates the node annotations between deployments so
that each of them can store metadata independently. The instance name must
start and end with an alphanumeric character and may only contain alphanumeric
characters, `-`, `_` or `.`.

Default: *empty*

Example:

```bash
nfd-master --instance=network
```

### --ca-file

The `--ca-file` is one of the three flags (together with `--cert-file` and
`--key-file`) controlling master-worker mutual TLS authentication on the
nfd-master side. This flag specifies the TLS root certificate that is used for
authenticating incoming connections. NFD-Worker side needs to have matching key
and cert files configured in order for the incoming requests to be accepted.

Default: *empty*

Note: Must be specified together with `--cert-file` and `--key-file`

Example:

```bash
nfd-master --ca-file=/opt/nfd/ca.crt --cert-file=/opt/nfd/master.crt --key-file=/opt/nfd/master.key
```

### --cert-file

The `--cert-file` is one of the three flags (together with `--ca-file` and
`--key-file`) controlling master-worker mutual TLS authentication on the
nfd-master side. This flag specifies the TLS certificate presented for
authenticating outgoing traffic towards nfd-worker.

Default: *empty*

Note: Must be specified together with `--ca-file` and `--key-file`

Example:

```bash
nfd-master --cert-file=/opt/nfd/master.crt --key-file=/opt/nfd/master.key --ca-file=/opt/nfd/ca.crt
```

### --key-file

The `--key-file` is one of the three flags (together with `--ca-file` and
`--cert-file`) controlling master-worker mutual TLS authentication on the
nfd-master side. This flag specifies the private key corresponding the given
certificate file (`--cert-file`) that is used for authenticating outgoing
traffic.

Default: *empty*

Note: Must be specified together with `--cert-file` and `--ca-file`

Example:

```bash
nfd-master --key-file=/opt/nfd/master.key --cert-file=/opt/nfd/master.crt --ca-file=/opt/nfd/ca.crt
```

### --verify-node-name

The `--verify-node-name` flag controls the NodeName based authorization of
incoming requests and only has effect when mTLS authentication has been enabled
(with `--ca-file`, `--cert-file` and `--key-file`). If enabled, the worker node
name of the incoming must match with the CN in its TLS certificate. Thus,
workers are only able to label the node they are running on (or the node whose
certificate they present), and, each worker must have an individual
certificate.

Node Name based authorization is disabled by default and thus it is possible
for all nfd-worker pods in the cluster to use one shared certificate, making
NFD deployment much easier.

Default: *false*

Example:

```bash
nfd-master --verify-node-name --ca-file=/opt/nfd/ca.crt \
    --cert-file=/opt/nfd/master.crt --key-file=/opt/nfd/master.key
```

### --no-publish

The `--no-publish` flag disables all communication with the Kubernetes API
server, making a "dry-run" flag for nfd-master. No Labels, Annotations or
ExtendedResources (or any other properties of any Kubernetes API objects) are
modified.

Default: *false*

Example:

```bash
nfd-master --no-publish
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
nfd-master --label-whitelist='.*cpuid\.'
```

### --extra-label-ns

The `--extra-label-ns` flag specifies a comma-separated list of allowed feature
label namespaces. By default, nfd-master only allows creating labels in the
default `feature.node.kubernetes.io` label namespace. This option can be used
to allow vendor-specific namespaces for custom labels from the local and custom
feature sources.

The same namespace control and this flag applies Extended Resources (created
with `--resource-labels`), too.

Default: *empty*

Example:

```bash
nfd-master --extra-label-ns=vendor-1.com,vendor-2.io
```

### --resource-labels

The `--resource-labels` flag specifies a comma-separated list of features to be
advertised as extended resources instead of labels. Features that have integer
values can be published as Extended Resources by listing them in this flag.

Default: *empty*

Example:

```bash
nfd-master --resource-labels=vendor-1.com/feature-1,vendor-2.io/feature-2
```
