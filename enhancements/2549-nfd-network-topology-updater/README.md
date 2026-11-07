# KEP-2549: NFD Network Topology Updater

## Summary

Add an optional NFD component named `nfd-network-topology-updater` that discovers
network topology in which nodes are connected in a specific pattern, for example,
fully connected cliques, maps it to Kubernetes nodes, and keeps the
corresponding NFD resources up to date.

This topology is represented as network groups: sets of nodes that share a
communication capability, each with a type, source, unique identity, and
membership. The model does not depend on the underlying interconnect technology.
Publish network group membership through NFD features and one `NodeFeatureGroup`
per network group, supporting multiple types and overlapping memberships.
Schedulers and DRA drivers can consume these groups to place distributed
multi-node workloads.

Hierarchical (tiered) backend switch fabric is intentionally out of scope. Node labels already
represent it fully, and existing consumers such as pod affinity and schedulers
already depend on those labels, so it is not published through NFD. See
[Scope: Network Groups, Not Switch Fabric](#scope-network-groups-not-switch-fabric).

## Motivation

AI, HPC, and distributed workloads need groups of nodes with suitable
communication capabilities. Nodes can be grouped in ways other than a hierarchical
topology, such as fully connected or overlapping groups, over a variety of interconnect
technologies. A node can belong to several network groups of different types, and
memberships can overlap. Node labels express this poorly: a label key holds a single value,
network group identifiers may not fit label value constraints, and a label cannot list
the members of a group. Deployment-specific integrations make network group information
difficult to discover and consume consistently.

Existing `NodeFeature` and `NodeFeatureGroup` APIs can publish network group information
without a new CRD. This complements `nfd-topology-updater`, which publishes
node-local resource and NUMA topology as `NodeResourceTopology`.

Hierarchical (tiered) backend switch fabric is a different case, and this KEP
deliberately does not use NFD for it. A tiered fabric is a hierarchical topology in
which every node belongs to exactly one domain per tier, so one Node label per
tier describes it completely. Publishing
it through NFD would offer no new capability and has three costs:

- **Duplicate information.** The same tier and locality data would exist in Node
  labels, `NodeFeature` attributes, and `NodeFeatureGroup` rules and status.
- **Extra resources.** Every tier and locality value would add objects that
  must be stored and that `nfd-master` re-evaluates whenever a `NodeFeature`
  changes, consuming API server storage and `nfd-master` CPU.
- **Backward compatibility.** Pod affinity and anti-affinity, topology spread
  constraints, and schedulers that already compare Node label values through a
  `topologyKey` cannot read `NodeFeatureGroup` objects. Moving the fabric into
  groups would require changing those consumers, and keeping the labels as well
  would bring back the duplication.

Fabric tier labels are already published by existing tools and remain the
interface for fabric locality.

## Goals And Non-Goals

### Goals

- Provide an optional updater and deployment, disabled by default, with support
  for multiple discovery sources.
- Publish per-node network group membership features and one `NodeFeatureGroup`
  per typed network group using existing NFD APIs.
- Keep membership current, clean up managed stale state, and avoid unnecessary
  writes or interference with other publishers.
- Support consumption of network groups by schedulers and DRA drivers.
- Complement, rather than replace, label-based fabric topology: leave tiered
  fabric on Node labels so existing consumers such as pod affinity and
  label-based schedulers keep working unchanged.

### Non-Goals

- Publishing backend switch fabric tiers through NFD features, groups, or Node
  labels. Fabric tier labels remain the responsibility of existing tools.
- Changing existing NFD API schemas or introducing a network group CRD.
- Replacing `nfd-topology-updater` or making network discovery mandatory.
- Implementing scheduling policy, a DRA driver, or ResourceSlice management.
- Supporting every discovery source in the first implementation.

## Proposal

### Scope: Network Groups, Not Switch Fabric

A backend switch fabric is usually organized in tiers. A group of compute nodes
connects to one or more switches, often called leaf switches. Those switches
connect to a set of switches at the next tier, often called spine switches, and so
on. Switches that share the same set of children (compute nodes or lower-tier
switches) and the same set of parents (upper-tier switches) are equivalent for
locality purposes and are represented as a single domain. The fabric then forms
a hierarchical topology in which every compute node has exactly one value per tier,
which one Node label per tier expresses completely. Native scheduling consumes such labels
directly, for example as a `topologyKey`.

Publishing this hierarchical topology through NFD would duplicate those labels and add
`NodeFeature` and `NodeFeatureGroup` objects that `nfd-master` must store and
re-evaluate on every change, without adding any capability. Tools such as
[Topograph](https://github.com/dsx-ai-factory/topograph/blob/main/docs/reference/node-labels.md)
already publish fabric tier labels and can continue to do so.

Network groups are different. Memberships are typed, can overlap, and identifiers can be
long, so labels cannot represent them without an unbounded number of keys.
`NodeFeatureGroup` represents any set of nodes and lists its members in status.

This KEP therefore proposes a hybrid: Node labels for tiered fabric, outside the
scope of the updater, and `NodeFeatureGroup` for network groups. Consumers can combine
both. Network group memberships may coincide with or overlap fabric locality groups.

### Use Cases

The three use cases below show which mechanism fits which topology. Label keys,
group types, and annotation names are illustrative.

#### Use Case 1: Tiered Fabric (Leaf, Spine, Core): Labels Only

Every node belongs to exactly one domain per tier, so one Node label per tier
describes the topology completely. Cloud providers and tools such as
[Topograph](https://github.com/dsx-ai-factory/topograph/blob/main/docs/reference/node-labels.md)
already publish these labels (`{a,b}` is shorthand for one label per value):

| Provider | Node labels |
| --- | --- |
| AWS (EKS) | `topology.k8s.aws/network-node-layer-{1,2,3}` |
| GCP (GKE) | `cloud.google.com/gce-topology-{block,subblock,host}` |
| OCI (OKE) | `oci.oraclecloud.com/rdma.{hpc_island_id,network_block_id,local_block_id}` |
| Topograph | `fabric.topograph.run/tier-{1,2,3}` |

```yaml
# Node worker-a: one value per tier
labels:
  fabric.topograph.run/tier-3: core-1
  fabric.topograph.run/tier-2: spine-4
  fabric.topograph.run/tier-1: leaf-17
```

Pod affinity, topology spread constraints, and label-based schedulers consume
these labels directly through `topologyKey`. The updater publishes nothing for
this case: no `NodeFeature`, no `NodeFeatureGroup`.

#### Use Case 2: Overlapping Accelerated Cliques: NodeFeatureGroup Only

Nodes are connected in fully connected cliques, for example accelerator
interconnect domains. A node can belong to several cliques, possibly of different
types, and cliques can overlap. A label key holds one value and cannot list
members, so labels do not fit. Each clique is a network group, published as a
`network.groups` instance on the node's `NodeFeature` and as one
`NodeFeatureGroup`.

```yaml
# NodeFeature for worker-a: member of two cliques
instances:
  network.groups:
    elements:
      - attributes: {source: example-provider, type: accelerated-clique, guid: clique-1}
      - attributes: {source: example-provider, type: accelerated-clique, guid: clique-2}
```

```yaml
# One NodeFeatureGroup per clique (clique-2 is analogous)
kind: NodeFeatureGroup
metadata:
  name: network-topology-clique-1
  annotations:
    network-topology.nfd.k8s-sigs.io/group-type: accelerated-clique
    network-topology.nfd.k8s-sigs.io/group-guid: clique-1
spec:
  featureGroupRules:
    - name: clique-1 members
      matchFeatures:
        - feature: network.groups
          matchExpressions:
            type: {op: In, value: [accelerated-clique]}
            guid: {op: In, value: [clique-1]}
```

`nfd-master` maintains `status.nodes` for each group. Schedulers and DRA drivers
read the groups to place multi-node workloads inside one clique.

#### Use Case 3: Dragonfly: Labels And NodeFeatureGroup

A dragonfly has nodes attached to routers, fully connected routers inside a
group, and groups connected by global links. Typical sizes are tens to a few
hundred groups with about 32 routers per group, so thousands of routers in
total. Locality is hierarchical, so labels carry it:

```yaml
# Node worker-a
labels:
  network.example.com/dragonfly-router: g7-r3   # unique cluster-wide
  network.example.com/dragonfly-group: g7
```

Router locality stays on labels only. A `NodeFeatureGroup` per router would add
thousands of objects that `nfd-master` re-evaluates on every `NodeFeature`
change, with no new capability. One `NodeFeatureGroup` per dragonfly group adds
what labels cannot provide: a member list and count without querying all Nodes,
and a named set that combines with other features, such as accelerator model.
The group id is also published in the node's `NodeFeature`, because groups match
`NodeFeature` data and not Node labels:

```yaml
instances:
  network.groups:
    elements:
      - attributes: {source: example-provider, type: dragonfly-group, guid: g7}
```

```yaml
kind: NodeFeatureGroup
metadata:
  name: network-topology-dragonfly-g7
  annotations:
    network-topology.nfd.k8s-sigs.io/group-type: dragonfly-group
    network-topology.nfd.k8s-sigs.io/group-guid: g7
    # Optional, sparse: only links that differ from the fabric default
    network-topology.nfd.k8s-sigs.io/group-links: "g8:hops=1,bandwidth=400Gb/s"
spec:
  featureGroupRules:
    - name: dragonfly g7 members
      matchFeatures:
        - feature: network.groups
          matchExpressions:
            type: {op: In, value: [dragonfly-group]}
            guid: {op: In, value: [g7]}
```

Consumers compute locality by comparing labels: equal router means the same
router, equal group means one local router hop, and different groups use a global
link. In a canonical dragonfly all groups are directly connected, so no pairwise
link data is needed. Irregular fabrics can record exceptions in the optional
per-group `group-links` annotation, which keeps link data in small per-group
objects instead of one matrix object. Each link is written on both groups. A
dedicated link-characteristics API is out of scope for this KEP.

### Discovery And Ownership

Network group discovery and publication can be performed by external services. The
publisher creates and updates `NodeFeature` objects and `NodeFeatureGroup`
metadata and rules. `nfd-master` evaluates the features and group rules and
maintains `NodeFeatureGroup.status.nodes`. It does not discover cluster network
topology.

The proposed updater provides this integration as an optional cluster-level NFD
component. External services can also publish directly through the same APIs.
Each resource has one publisher; the updater must not adopt or overwrite objects
owned by another service.

Discovery returns independent network group records with source, type, identity, and
complete membership. Network group memberships must not be reduced to one group per node
or forced into a hierarchy.

### Published Features

Publish one managed `NodeFeature` per node with discovered network group membership.
Network group memberships use `network.groups` instance attributes so a node can belong
to multiple network groups of different types. The feature name and attribute
conventions below are illustrative and require agreement during implementation.

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeature
metadata:
  name: network-topology-worker-a
  namespace: node-feature-discovery
  labels:
    app.kubernetes.io/managed-by: nfd-network-topology-updater
    nfd.node.kubernetes.io/node-name: worker-a
spec:
  features:
    attributes:
      system.name:
        elements:
          nodename: worker-a
    instances:
      network.groups:
        elements:
          - attributes:
              source: example-provider
              type: mle
              guid: group-123
```

### Published Groups And Consumers

Create one `NodeFeatureGroup` per network group. Each group matches source, type,
and identity within the same membership instance:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: network-topology-group-123
  namespace: node-feature-discovery
  labels:
    app.kubernetes.io/managed-by: nfd-network-topology-updater
  annotations:
    network-topology.nfd.k8s-sigs.io/group-source: example-provider
    network-topology.nfd.k8s-sigs.io/group-type: mle
    network-topology.nfd.k8s-sigs.io/group-guid: group-123
spec:
  featureGroupRules:
    - name: mle group members
      matchFeatures:
        - feature: network.groups
          matchExpressions:
            source:
              op: In
              value: [example-provider]
            type:
              op: In
              value: [mle]
            guid:
              op: In
              value: [group-123]
```

Generate stable Kubernetes-safe names using network group source and identity, with
deterministic hashes where needed. Store full network group identifiers in annotations
rather than constrained label values.

Schedulers and DRA drivers can consume the groups. Group publication requires the
existing `NodeFeatureGroupAPI` feature gate and CRD.

Group status lists nodes only. If a consumer needs individual accelerator
membership, the corresponding `NodeFeature` instances can carry stable device
identifiers. Node inclusion must not imply that every device belongs to a network group;
the device-level contract is an open design question below.

### Reconciliation And Deployment

Run the updater as a Deployment, with leader election when using multiple
replicas. Refresh discovery periodically or through source notifications, validate
results, and reconcile only changed features and group specifications. Cleanup
must be restricted to publisher-owned resources. Failed discovery must not be
treated as authoritative deletion or leave unusable network group membership presented
as current.

Provide Helm and Kustomize installation options. Grant the publisher access to
Nodes, managed features and groups, and configured provider credentials. Give
consumers read-only topology access; DRA drivers retain their own ResourceSlice
permissions. Document cleanup when disabling the updater.

## Open Questions And Risks

- **Device membership.** Agree on the instance attributes and semantics needed by
  DRA consumers when only some accelerators on a node belong to a network group.
- **Freshness and consistency.** Feature updates and group-status evaluation are
  asynchronous. Define how consumers detect incomplete discovery, partially
  published membership, and stopped publishers before using topology for placement.
- **Discovery scope.** Define how node selection and publication scope are exposed
  so filtered membership is not mistaken for complete cluster-wide discovery.
- **Scale and trust.** Large overlapping groups can amplify status and API writes.
  Skip no-op updates and restrict publisher permissions. Group consumers depend on
  the alpha NodeFeatureGroup API.

## Test Plan

- Test multiple network group types, overlapping memberships, stable naming, and
  same-instance network group matching.
- Test changed membership, discovery failures, cleanup ownership, no-op updates,
  and delayed group-status evaluation.
- Add end-to-end tests for populated network group status; cover device
  membership once its contract is agreed.
- Verify installation manifests, RBAC, and read-only consumer access.

## Alternatives And Candidate Sources

Keeping discovery external avoids another NFD component but requires users to
operate an integration. Publishing network groups only as Node labels cannot express
overlapping, typed memberships with long identifiers, and does not provide the
same feature and group interfaces. A dedicated CRD for network groups or the full
network could expose more detail, but adds a new API; reuse existing NFD resources
initially.

Publishing the backend switch fabric through NFD features, groups, and labels was
considered and rejected. A tiered fabric is a hierarchical topology with one value
per tier per node, so Node labels represent it fully; NFD resources would only duplicate them
and add storage and `nfd-master` evaluation cost.

Sources may include fabric management tools, cloud APIs, and accelerator APIs.
[Topograph](https://github.com/dsx-ai-factory/topograph) is one candidate source
and implementation starting point. Its NFD engine already creates per-node
features and groups for accelerator domains, with `nfd-master` maintaining group
status. Reuse its accelerator discovery and extend network group publication to preserve
multiple typed memberships through existing NFD resources.

Public references:

- [Topograph Node labels](https://github.com/dsx-ai-factory/topograph/blob/main/docs/reference/node-labels.md)
- [Topograph NFD engine](https://github.com/dsx-ai-factory/topograph/blob/main/docs/engines/nfd.md)
