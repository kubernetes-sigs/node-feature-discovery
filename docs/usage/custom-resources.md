---
title: "CRDs"
layout: default
sort: 7
---

# Custom Resources
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

NFD uses some Kubernetes [custom resources][custom-resources].

## NodeFeature

**EXPERIMENTAL**
NodeFeature is an NFD-specific custom resource for communicating node
features and node labeling requests. Support for NodeFeature objects is
disabled by default. If enabled, nfd-master watches for NodeFeature objects,
labels nodes as specified and uses the listed features as input when evaluating
[NodeFeatureRule](#nodefeaturerule)s. NodeFeature objects can be used for
implementing 3rd party extensions (see
[customization guide](customization-guide#nodefeature-custom-resource) for more
details).

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeature
metadata:
  labels:
    nfd.node.kubernetes.io/node-name: node-1
  name: node-1-vendor-features
spec:
  features:
    instances:
      vendor.device:
        elements:
        - attributes:
            model: "xpu-1"
            memory: "4000"
            type: "fast"
        - attributes:
            model: "xpu-2"
            memory: "16000"
            type: "slow"
  labels:
    vendor-xpu-present: "true"
```

## NodeFeatureRule

NodeFeatureRule is an NFD-specific custom resource that is designed for
rule-based custom labeling of nodes. NFD-Master watches for NodeFeatureRule
objects in the cluster and labels nodes according to the rules within. Some use
cases are e.g. application specific labeling in a specific environments or
being distributed by hardware vendors to create specific labels for their
devices.

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: example-rule
spec:
  rules:
    - name: "example rule"
      labels:
        "example-custom-feature": "true"
      # Label is created if all of the rules below match
      matchFeatures:
        # Match if "veth" kernel module is loaded
        - feature: kernel.loadedmodule
          matchExpressions:
            veth: {op: Exists}
        # Match if any PCI device with vendor 8086 exists in the system
        - feature: pci.device
          matchExpressions:
            vendor: {op: In, value: ["8086"]}
```

See the
[Customization guide](customization-guide#node-feature-rule-custom-resource)
for full documentation of the NodeFeatureRule resource and its usage.

## NodeResourceTopology

When run with NFD-Topology-Updater, NFD creates NodeResourceTopology objects
corresponding to node resource hardware topology such as:

```yaml
apiVersion: topology.node.k8s.io/v1alpha1
kind: NodeResourceTopology
metadata:
  name: node1
topologyPolicies: ["SingleNUMANodeContainerLevel"]
zones:
  - name: node-0
    type: Node
    resources:
      - name: cpu
        capacity: 20
        allocatable: 16
        available: 10
      - name: vendor/nic1
        capacity: 3
        allocatable: 3
        available: 3
  - name: node-1
    type: Node
    resources:
      - name: cpu
        capacity: 30
        allocatable: 30
        available: 15
      - name: vendor/nic2
        capacity: 6
        allocatable: 6
        available: 6
  - name: node-2
    type: Node
    resources:
      - name: cpu
        capacity: 30
        allocatable: 30
        available: 15
      - name: vendor/nic1
        capacity: 3
        allocatable: 3
        available: 3
```

The NodeResourceTopology objects created by NFD can be used to gain insight
into the allocatable resources along with the granularity of those resources at
a per-zone level (represented by node-0 and node-1 in the above example) or can
be used by an external entity (e.g. topology-aware scheduler plugin) to take an
action based on the gathered information.

<!-- Links -->
[custom-resources]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
