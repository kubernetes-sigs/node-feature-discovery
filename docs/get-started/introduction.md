---
title: "Introduction"
layout: default
sort: 1
---

# Introduction
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

This software enables node feature discovery for Kubernetes. It detects
hardware features available on each node in a Kubernetes cluster, and
advertises those features using node labels.

NFD consists of two software components:

1. nfd-master
1. nfd-worker

## NFD-Master

NFD-Master is the daemon responsible for communication towards the Kubernetes
API. That is, it receives labeling requests from the worker and modifies node
objects accordingly.

## NFD-Worker

NFD-Worker is a daemon responsible for feature detection. It then communicates
the information to nfd-master which does the actual node labeling.  One
instance of nfd-worker is supposed to be running on each node of the cluster,

## Feature Discovery

Feature discovery is divided into domain-specific feature sources:

- CPU
- IOMMU
- Kernel
- Memory
- Network
- PCI
- Storage
- System
- USB
- Custom (rule-based custom features)
- Local (hooks for user-specific features)

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
  "feature.node.kubernetes.io/iommu-<feature-name>": "true",
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

## Node Annotations

NFD also annotates nodes it is running on:

| Annotation                                | Description
| ----------------------------------------- | -----------
| nfd.node.kubernetes.io/master.version     | Version of the nfd-master instance running on the node. Informative use only.
| nfd.node.kubernetes.io/worker.version     | Version of the nfd-worker instance running on the node. Informative use only.
| nfd.node.kubernetes.io/feature-labels     | Comma-separated list of node labels managed by NFD. NFD uses this internally so must not be edited by users.
| nfd.node.kubernetes.io/extended-resources | Comma-separated list of node extended resources managed by NFD. NFD uses this internally so must not be edited by users.

Unapplicable annotations are not created, i.e. for example master.version is only created on nodes running nfd-master.

