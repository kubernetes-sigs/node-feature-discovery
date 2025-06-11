---
title: "Topology-Updater config reference"
parent: "Reference"
layout: default
nav_order: 6
---

# Configuration file reference of nfd-topology-updater
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

See the
[sample configuration file](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/topology-updater-config/nfd-topology-updater.conf.example)
for a full example configuration.

## excludeList

The `excludeList` specifies a key-value map of allocated resources
that should not be examined by the topology-updater
agent per node.
Each key is a node name with a value as a list of resources
that should not be examined by the agent for that specific node.

Default: *empty*

Example:

```yaml
excludeList:
  nodeA: [hugepages-2Mi]
  nodeB: [memory]
  nodeC: [cpu, hugepages-2Mi]
```

### excludeList.*
`excludeList.*` is a special value that use to specify all nodes.
A resource that would be listed under this key, would be excluded from all nodes.

Default: *empty*

Example:

```yaml
excludeList:
  '*': [hugepages-2Mi]
```
