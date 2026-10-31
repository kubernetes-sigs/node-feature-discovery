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
