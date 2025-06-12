---
title: "Introduction"
parent: "Get started"
layout: default
nav_order: 1
---

# Introduction
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

This software enables node feature discovery for Kubernetes. It detects
hardware features available on each node in a Kubernetes cluster, and
advertises those features using node labels and optionally node extended
resources, annotations and node taints. Node Feature Discovery is compatible
with any recent version of Kubernetes (v1.24+).

NFD consists of four software components:

1. nfd-master
1. nfd-worker
1. nfd-topology-updater
1. nfd-gc

## NFD-Master

NFD-Master is the daemon responsible for communication towards the Kubernetes
API. That is, it receives labeling requests from the worker and modifies node
objects accordingly.

## NFD-Worker

NFD-Worker is a daemon responsible for feature detection. It then communicates
the information to nfd-master which does the actual node labeling.  One
instance of nfd-worker is supposed to be running on each node of the cluster,

## NFD-Topology-Updater

NFD-Topology-Updater is a daemon responsible for examining allocated
resources on a worker node to account for resources available to be allocated
to new pod on a per-zone basis (where a zone can be a NUMA node). It then
creates or updates a
[NodeResourceTopology](../usage/custom-resources.md#noderesourcetopology) custom
resource object specific to this node. One instance of nfd-topology-updater is
supposed to be running on each node of the cluster.

## NFD-GC

NFD-GC is a daemon responsible for cleaning obsolete
[NodeFeature](../usage/custom-resources.md#nodefeature) and
[NodeResourceTopology](../usage/custom-resources.md#noderesourcetopology) objects.

One instance of nfd-gc is supposed to be running in the cluster.

## Feature Discovery

Feature discovery is divided into domain-specific feature sources:

- CPU
- Kernel
- Memory
- Network
- PCI
- Storage
- System
- USB
- Custom (rule-based custom features)
- Local (features files)

Each feature source is responsible for detecting a set of features which. in
turn, are turned into node feature labels.  Feature labels are prefixed with
`feature.node.kubernetes.io/` and also contain the name of the feature source.
Non-standard user-specific feature labels can be created with the local and
custom feature sources.

An overview of the default feature labels:

```json
{
  "feature.node.kubernetes.io/cpu-<feature-name>": "true",
  "feature.node.kubernetes.io/custom-<feature-name>": "true",
  "feature.node.kubernetes.io/kernel-<feature name>": "<feature value>",
  "feature.node.kubernetes.io/memory-<feature-name>": "true",
  "feature.node.kubernetes.io/network-<feature-name>": "true",
  "feature.node.kubernetes.io/pci-<device label>.present": "true",
  "feature.node.kubernetes.io/storage-<feature-name>": "true",
  "feature.node.kubernetes.io/system-<feature name>": "<feature value>",
  "feature.node.kubernetes.io/usb-<device label>.present": "<feature value>",
  "feature.node.kubernetes.io/<file name>-<feature name>": "<feature value>"
}
```

## Node annotations

NFD also annotates nodes it is running on:

| Annotation                                                    | Description                                                 |
| ------------------------------------------------------------- | ----------------------------------------------------------- |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/feature-labels      | Comma-separated list of node labels managed by NFD. NFD uses this internally so must not be edited by users. |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/feature-annotations | Comma-separated list of node annotations managed by NFD. NFD uses this internally so must not be edited by users. |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/extended-resources  | Comma-separated list of node extended resources managed by NFD. NFD uses this internally so must not be edited by users. |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/taints              | Comma-separated list of node taints managed by NFD. NFD uses this internally so must not be edited by users. |

> **NOTE:** the [`-instance`](../reference/master-commandline-reference.md#instance)
> command line flag affects the annotation names

Unapplicable annotations are not created, i.e. for example
`nfd.node.kubernetes.io/extended-resources` is only placed if some extended
resources were created by NFD.

## Custom resources

NFD takes use of some Kubernetes Custom Resources.

[NodeFeature](../usage/custom-resources.md#nodefeature)s
is be used for representing node features and requesting node labels to be
generated.

NFD-Master uses [NodeFeatureRule](../usage/custom-resources.md#nodefeaturerule)s
for custom labeling of nodes.

NFD-Topology-Updater creates
[NodeResourceTopology](../usage/custom-resources.md#noderesourcetopology) objects
that describe the hardware topology of node resources.
