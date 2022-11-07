---
title: "Topology Updater Cmdline Reference"
layout: default
sort: 4
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
docker run gcr.io/k8s-staging-nfd/node-feature-discovery:master nfd-topology-updater -help
```

### -h, -help

Print usage and exit.

### -version

Print version and exit.

### -server

The `-server` flag specifies the address of the nfd-master endpoint where to
connect to.

Default: localhost:8080

Example:

```bash
nfd-topology-updater -server=nfd-master.nfd.svc.cluster.local:443
```

### -ca-file

The `-ca-file` is one of the three flags (together with `-cert-file` and
`-key-file`) controlling the mutual TLS authentication on the topology-updater side.
This flag specifies the TLS root certificate that is used for verifying the
authenticity of nfd-master.

Default: *empty*

Note: Must be specified together with `-cert-file` and `-key-file`

Example:

```bash
nfd-topology-updater -ca-file=/opt/nfd/ca.crt -cert-file=/opt/nfd/updater.crt -key-file=/opt/nfd/updater.key
```

### -cert-file

The `-cert-file` is one of the three flags (together with `-ca-file` and
`-key-file`) controlling mutual TLS authentication on the topology-updater
side. This flag specifies the TLS certificate presented for authenticating
outgoing requests.

Default: *empty*

Note: Must be specified together with `-ca-file` and `-key-file`

Example:

```bash
nfd-topology-updater -cert-file=/opt/nfd/updater.crt -key-file=/opt/nfd/updater.key -ca-file=/opt/nfd/ca.crt
```

### -key-file

The `-key-file` is one of the three flags (together with `-ca-file` and
`-cert-file`) controlling the mutual TLS authentication on topology-updater
side. This flag specifies the private key corresponding the given certificate file
(`-cert-file`) that is used for authenticating outgoing requests.

Default: *empty*

Note: Must be specified together with `-cert-file` and `-ca-file`

Example:

```bash
nfd-topology-updater -key-file=/opt/nfd/updater.key -cert-file=/opt/nfd/updater.crt -ca-file=/opt/nfd/ca.crt
```

### -server-name-override

The `-server-name-override` flag specifies the common name (CN) which to
expect from the nfd-master TLS certificate. This flag is mostly intended for
development and debugging purposes.

Default: *empty*

Example:

```bash
nfd-topology-updater -server-name-override=localhost
```

### -no-publish

The `-no-publish` flag disables all communication with the nfd-master, making
it a "dry-run" flag for nfd-topology-updater. NFD-Topology-Updater runs
resource hardware topology detection normally, but no CR requests are sent to
nfd-master.

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

### -sleep-interval

The `-sleep-interval` specifies the interval between resource hardware
topology re-examination (and CR updates). A non-positive value implies
infinite sleep interval, i.e. no re-detection is done.

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
Note that the URi could either be a local host file or an HTTP endpoint.

Default:  `https://${NODE_NAME}:10250/configz`

Example:

```bash
nfd-topology-updater -kubelet-config-uri=file:///var/lib/kubelet/config.yaml
```

### -api-auth-token-file

The `-api-auth-token-file` specifies the path to the api auth token file
which is used to retrieve Kubelet's configuration from Kubelet secure port,
only taking effect when `-kubelet-config-uri` is https.
Note that this token file must bind to a role that has the `get` capability to
`nodes/proxy` resources.

Default:  `/var/run/secrets/kubernetes.io/serviceaccount/token`

Example:

```bash
nfd-topology-updater -token-file=/var/run/secrets/kubernetes.io/serviceaccount/token
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
