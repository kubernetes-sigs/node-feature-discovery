---
title: "NFD-Worker"
parent: "Usage"
layout: default
nav_order: 4
---

# NFD-Worker
{: .no_toc}

---

NFD-Worker is preferably run as a Kubernetes DaemonSet. This assures
re-labeling on regular intervals capturing changes in the system configuration
and makes sure that new nodes are labeled as they are added to the cluster.
Worker connects to the nfd-master service to advertise hardware features.

When run as a daemonset, nodes are re-labeled at an default interval of 60s.
This can be changed by using the
[`core.sleepInterval`](../reference/worker-configuration-reference.md#coresleepinterval)
config option.

## Worker configuration

NFD-Worker supports configuration through a configuration file. The
default location is `/etc/kubernetes/node-feature-discovery/nfd-worker.conf`,
but, this can be changed by specifying the`-config` command line flag.
Configuration file is re-read whenever it is modified which makes run-time
re-configuration of nfd-worker straightforward.

Worker configuration file is read inside the container, and thus, Volumes and
VolumeMounts are needed to make your configuration available for NFD. The
preferred method is to use a ConfigMap which provides easy deployment and
re-configurability.

The provided deployment methods (Helm and Kustomize) create an empty configmap
and mount it inside the nfd-master containers.

In Helm deployments,
[Worker pod parameter](../deployment/helm.md#worker-pod-parameters)
`worker.config` can be used to edit the respective configuration.

In Kustomize deployments, modify the `nfd-worker-conf` ConfigMap with a custom
overlay.

> **NOTE:** dynamic run-time reconfiguration was dropped in NFD v0.17.
> Re-configuration is handled by pod restarts.

See
[nfd-worker configuration file reference](../reference/worker-configuration-reference)
for more details.
The (empty-by-default)
[example config](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/worker-config/nfd-worker.conf.example)
contains all available configuration options and can be used as a reference
for creating a configuration.

Configuration options can also be specified via the `-options` command line
flag, in which case no mounts need to be used. The same format as in the config
file must be used, i.e. JSON (or YAML). For example:

```bash
-options='{"sources": { "pci": { "deviceClassWhitelist": ["12"] } } }'
```

Configuration options specified from the command line will override those read
from the config file.
