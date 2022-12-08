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
