---
title: "Introduction"
layout: default
sort: 1
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

In addition, NFD ships two client tools: the
[`kubectl-nfd`](../usage/kubectl-plugin.md) kubectl plugin for validating and
testing NodeFeatureRules, and the `nfd` command line tool for
[exporting features and labels](../usage/nfd-export.md) and checking
[image compatibility](../usage/image-compatibility.md).

## NFD-Master

NFD-Master is the daemon responsible for updating node objects. It watches the
NodeFeature objects published by nfd-worker (and other agents) and the
NodeFeatureRule objects, and modifies node labels, annotations, extended
resources and taints accordingly.

## NFD-Worker

NFD-Worker is a daemon responsible for feature detection. It publishes the
detected features as a NodeFeature object in the Kubernetes API, from which
nfd-master does the actual node labeling. One instance of nfd-worker is
supposed to be running on each node of the cluster.

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

Each feature source is responsible for detecting a set of features which, in
turn, are turned into node feature labels. Labels from the built-in sources are
named `feature.node.kubernetes.io/<source name>-<feature name>` (for example
`feature.node.kubernetes.io/cpu-cpuid.AESNI`). Non-standard user-specific
feature labels can be created with the local and custom feature sources. Their
labels are used exactly as named in the feature file or rule
(`<namespace>/<name>`), with no source or file-name prefix. Unprefixed names
are currently auto-prefixed with `feature.node.kubernetes.io/` (see the
[DisableAutoPrefix](../reference/feature-gates.md#disableautoprefix) feature
gate).

An overview of the default feature labels:

```json
{
  "feature.node.kubernetes.io/cpu-<feature-name>": "true",
  "<namespace>/<label name from custom rule>": "<label value>",
  "feature.node.kubernetes.io/kernel-<feature name>": "<feature value>",
  "feature.node.kubernetes.io/memory-<feature-name>": "true",
  "feature.node.kubernetes.io/network-<feature-name>": "true",
  "feature.node.kubernetes.io/pci-<device label>.present": "true",
  "feature.node.kubernetes.io/storage-<feature-name>": "true",
  "feature.node.kubernetes.io/system-<feature name>": "<feature value>",
  "feature.node.kubernetes.io/usb-<device label>.present": "true",
  "<namespace>/<feature name from features.d file>": "<feature value>"
}
```

## Node annotations

NFD also annotates nodes it is running on:

| Annotation                                                    | Description                                                 |
| ------------------------------------------------------------- | ----------------------------------------------------------- |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/feature-labels      | Comma-separated list of node labels managed by NFD. NFD uses this internally so must not be edited by users. |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/feature-annotations | Comma-separated list of node annotations managed by NFD. NFD uses this internally so must not be edited by users. |
| [&lt;instance&gt;.]nfd.node.kubernetes.io/extended-resources  | Comma-separated list of node extended resources managed by NFD. NFD uses this internally so must not be edited by users. |
| nfd.node.kubernetes.io/taints                                 | Comma-separated list of node taints managed by NFD. NFD uses this internally so must not be edited by users. Not affected by the -instance flag. |

> **NOTE:** the [`-instance`](../reference/master-commandline-reference.md#-instance)
> command line flag affects the names of the feature-labels,
> feature-annotations and extended-resources annotations.

Inapplicable annotations are not created, for example
`nfd.node.kubernetes.io/extended-resources` is only placed if some extended
resources were created by NFD.

## Custom resources

NFD makes use of some Kubernetes Custom Resources.

[NodeFeature](../usage/custom-resources.md#nodefeature)s
are used for representing node features and requesting node labels to be
generated.

NFD-Master uses [NodeFeatureRule](../usage/custom-resources.md#nodefeaturerule)s
for custom labeling of nodes.

NFD-Master also evaluates
[NodeFeatureGroup](../usage/custom-resources.md#nodefeaturegroup)s in its own
namespace (alpha, enabled with the
[`NodeFeatureGroupAPI`](../reference/feature-gates.md#nodefeaturegroupapi)
feature gate) and updates their status with the list of nodes that match the
group rules.

NFD-Topology-Updater creates
[NodeResourceTopology](../usage/custom-resources.md#noderesourcetopology) objects
that describe the hardware topology of node resources.
