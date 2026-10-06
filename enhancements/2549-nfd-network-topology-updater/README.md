# KEP-2549: NFD Network Topology Updater

## Summary

Add an optional NFD component named `nfd-network-topology-updater` that discovers
cluster network topology, maps it to Kubernetes nodes, and keeps the corresponding
NFD resources up to date.

The topology model represents two groups of information:

- **Backend switch fabric (tiers).** A variable-depth switch hierarchy. Tier 0
  identifies the switch closest to the compute node; higher numbers identify
  successive tiers outward. Publish all discovered tiers using NFD features and
  groups and `fabric.topograph.run/tier-N` Node labels.
- **Accelerator interconnects (cliques).** Communication-capability groups with a
  type, source, unique identity, and membership. Publish clique membership through
  NFD features and one `NodeFeatureGroup` per clique, supporting multiple types
  and overlapping memberships.

Fabric tiers and cliques can coexist, and their node memberships may coincide or
overlap. Both identify groups of nodes suitable for placing distributed multi-node
workloads and can be consumed by schedulers and DRA drivers. Discovery sources may
differ, but the placement purpose is the same.

## Motivation

AI, HPC, and distributed workloads need groups of nodes with suitable network
locality and communication capabilities. Deployment-specific integrations make
this information difficult to discover and consume consistently.

Existing `NodeFeature` and `NodeFeatureGroup` APIs can publish both fabric and
clique information without a new CRD. This complements `nfd-topology-updater`,
which publishes node-local resource and NUMA topology as `NodeResourceTopology`.

## Goals And Non-Goals

### Goals

- Provide an optional updater and deployment, disabled by default, with support
  for multiple discovery sources.
- Publish per-node topology features, fabric labels, and groups for both fabric
  locality and typed cliques using existing NFD APIs.
- Keep membership current, clean up managed stale state, and avoid unnecessary
  writes or interference with other publishers.
- Support consumption of both topology types by schedulers and DRA drivers.

### Non-Goals

- Changing existing NFD API schemas or introducing a Clique CRD.
- Replacing `nfd-topology-updater` or making network discovery mandatory.
- Implementing scheduling policy, a DRA driver, or ResourceSlice management.
- Standardizing Kubernetes-wide topology label keys or supporting every provider
  in the first implementation.

## Proposal

### Discovery And Ownership

Topology discovery and publication can be performed by external services. The
publisher creates and updates `NodeFeature` objects and `NodeFeatureGroup`
metadata and rules. `nfd-master` evaluates the features and group rules, maintains
`NodeFeatureGroup.status.nodes`, and applies requested Node labels under its
configured restrictions. It does not discover cluster network topology.

The proposed updater provides this integration as an optional cluster-level NFD
component. External services can also publish directly through the same APIs.
Each resource has one publisher; the updater must not adopt or overwrite objects
owned by another service.

Discovery returns each node's locality at every fabric tier and independent
clique records with source, type, identity, and complete membership. Clique
memberships must not be reduced to one accelerator domain or forced into the
fabric hierarchy.

### Published Features And Labels

Publish one managed `NodeFeature` per node with discovered topology. Fabric
attributes use `network.topology`; clique memberships use `network.cliques`
instance attributes so a node can belong to multiple cliques of different types.
The feature names and clique attribute conventions below are illustrative and
require agreement during implementation.

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
  labels:
    fabric.topograph.run/tier-0: switch-12
    fabric.topograph.run/tier-1: switch-2
  features:
    attributes:
      system.name:
        elements:
          nodename: worker-a
      network.topology:
        elements:
          fabric-tier-0: switch-12
          fabric-tier-1: switch-2
    instances:
      network.cliques:
        elements:
          - attributes:
              source: example-provider
              type: mle
              guid: clique-123
```

Request backend fabric labels through `NodeFeature.spec.labels`, using
`fabric.topograph.run/tier-N` for every discovered tier. Document NFD label
namespace permissions and restrictions such as `restrictions.denyNodeFeatureLabels`.
Encode values outside label constraints deterministically, preserve locality
equality, and retain original values in features. Remove obsolete managed labels.

### Published Groups And Consumers

Create one `NodeFeatureGroup` per distinct fabric tier-and-locality value and one
per clique. Fabric groups match `fabric-tier-N` attributes. Clique groups match
source, type, and identity within the same membership instance:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: network-topology-clique-123
  namespace: node-feature-discovery
  labels:
    app.kubernetes.io/managed-by: nfd-network-topology-updater
    network-topology.nfd.k8s-sigs.io/group-type: clique
  annotations:
    network-topology.nfd.k8s-sigs.io/clique-source: example-provider
    network-topology.nfd.k8s-sigs.io/clique-type: mle
    network-topology.nfd.k8s-sigs.io/clique-guid: clique-123
spec:
  featureGroupRules:
    - name: mle clique members
      matchFeatures:
        - feature: network.cliques
          matchExpressions:
            source:
              op: In
              value: [example-provider]
            type:
              op: In
              value: [mle]
            guid:
              op: In
              value: [clique-123]
```

Generate stable Kubernetes-safe names using tier and locality or clique source
and identity, with deterministic hashes where needed. Store full clique
identifiers in annotations rather than constrained label values.

Schedulers and DRA drivers can consume both kinds of groups. Native scheduling
can also compare fabric labels using a topology key such as
`fabric.topograph.run/tier-0`. Group publication requires the existing
`NodeFeatureGroupAPI` feature gate and CRD.

Group status lists nodes only. If a consumer needs individual accelerator
membership, the corresponding `NodeFeature` instances can carry stable device
identifiers. Node inclusion must not imply that every device belongs to a clique;
the device-level contract is an open design question below.

If [nested NodeFeatureGroup support](https://github.com/kubernetes-sigs/node-feature-discovery/pull/2551)
is adopted, higher-tier fabric groups can reference lower-tier children. Cliques
remain independent membership sets even when they coincide with fabric groups.

### Reconciliation And Deployment

Run the updater as a Deployment, with leader election when using multiple
replicas. Refresh discovery periodically or through source notifications, validate
results, and reconcile only changed features and group specifications. Cleanup
must be restricted to publisher-owned resources. Failed discovery must not be
treated as authoritative deletion or leave unusable clique membership presented
as current.

Provide Helm and Kustomize installation options. Grant the publisher access to
Nodes, managed features and groups, and configured provider credentials. Give
consumers read-only topology access; DRA drivers retain their own ResourceSlice
permissions. Document cleanup and label removal when disabling the updater.

## Open Questions And Risks

- **Device membership.** Agree on the instance attributes and semantics needed by
  DRA consumers when only some accelerators on a node belong to a clique.
- **Freshness and consistency.** Feature updates and group-status evaluation are
  asynchronous. Define how consumers detect incomplete discovery, partially
  published membership, and stopped publishers before using topology for placement.
- **Discovery scope.** Define how node selection and publication scope are exposed
  so filtered membership is not mistaken for complete cluster-wide discovery.
- **Scale and trust.** Large overlapping groups can amplify status and API writes.
  Skip no-op updates, restrict publisher permissions, and avoid competing owners
  for fabric labels. Group consumers depend on the alpha NodeFeatureGroup API.

## Test Plan

- Test variable-depth fabric labels and groups, multiple clique types, overlapping
  memberships, stable naming, and same-instance clique matching.
- Test changed membership, discovery failures, cleanup ownership, no-op updates,
  and delayed group-status evaluation.
- Add end-to-end tests for NFD-applied fabric labels and populated fabric and
  clique group status; cover device membership once its contract is agreed.
- Verify installation manifests, RBAC, and read-only consumer access.

## Alternatives And Candidate Sources

Keeping discovery external avoids another NFD component but requires users to
operate an integration. Publishing only Node labels does not provide the same
feature and group interfaces. A dedicated clique or full-network CRD could expose
more detail, but adds a new API; reuse existing NFD resources initially.

Sources may include fabric management tools, cloud APIs, and accelerator APIs.
[Topograph](https://github.com/dsx-ai-factory/topograph) is one candidate source
and implementation starting point. Its NFD engine already creates per-node
features and groups for fabric tiers and accelerator domains, with `nfd-master`
maintaining group status. Reuse its fabric discovery and
`fabric.topograph.run/tier-N` conventions, and extend clique publication to
preserve multiple typed memberships through existing NFD resources.

Public references:

- [Topograph Node labels](https://github.com/dsx-ai-factory/topograph/blob/main/docs/reference/node-labels.md)
- [Topograph NFD engine](https://github.com/dsx-ai-factory/topograph/blob/main/docs/engines/nfd.md)
