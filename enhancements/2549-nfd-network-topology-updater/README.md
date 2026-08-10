# KEP-2549: NFD Network Topology Updater

## Summary

Add a new optional NFD component named `nfd-network-topology-updater`.

The updater discovers cluster network topology, projects it into NFD feature
data, and maintains NFD custom resources that other components can consume. The
initial implementation is based on the Topograph project and, in particular,
its implemented NFD engine, which converts the canonical `topology.Graph` into
per-node topology attributes and `NodeFeatureGroup` objects. See the
[Topograph NFD engine documentation](https://github.com/NVIDIA/topograph/blob/main/docs/engines/nfd.md)
and its
[design document](https://github.com/NVIDIA/topograph/blob/main/docs/design/nfd-engine-sdd.md).

This component is separate from the existing `nfd-topology-updater`. The
existing updater publishes `NodeResourceTopology` objects that describe
node-local resource and NUMA topology. `nfd-network-topology-updater` publishes
cluster network fabric topology as variable-depth fabric tiers and optional
accelerator domain and sub-domain dimensions.

## Motivation

Large AI, HPC, and distributed storage clusters often need placement decisions
that account for the network fabric. Workloads may prefer nodes under the same
leaf switch, avoid crossing spine switches, or pack latency-sensitive replicas
inside the same accelerator network domain.

Today this information is usually exposed through deployment-specific labels or
through external tools such as Topograph. That makes it harder for NFD consumers
to discover topology consistently, and it leaves each integration to solve the
same problems: discovering topology, mapping fabric objects back to Kubernetes
nodes, keeping data fresh, and cleaning stale state.

NFD already provides two useful primitives for this problem:

- `NodeFeature` can publish node-scoped feature attributes from third-party
  discovery components.
- `NodeFeatureGroup` can materialize groups of nodes that share a feature value.

An NFD-owned network topology updater can use those primitives to expose
network topology in a familiar way, while reusing Topograph's graph discovery
and topology projection model.

### Goals

- Add an optional `nfd-network-topology-updater` binary and Kubernetes
  deployment.
- Base the initial topology discovery and graph projection on Topograph.
- Publish per-node network topology attributes as `NodeFeature` objects.
- Create one `NodeFeatureGroup` for each distinct value at every discovered
  network fabric tier and for each accelerator domain or sub-domain.
- Keep generated object names stable and Kubernetes-safe.
- Support cleanup of stale updater-managed objects.
- Avoid unnecessary writes when discovered topology has not changed.
- Provide Helm and Kustomize installation paths, disabled by default.
- Expose health and metrics endpoints consistent with other NFD components.

### Non-Goals

- Replacing the existing `nfd-topology-updater` or changing
  `NodeResourceTopology` behavior.
- Making network topology discovery required for NFD installations.
- Creating a new scheduler, scheduler plugin, or Kubernetes scheduling API.
- Standardizing Kubernetes-wide network topology label keys.
- Requiring all Topograph providers to be supported in the first
  implementation.
- Changing the `NodeFeature`, `NodeFeatureRule`, or `NodeFeatureGroup` APIs in
  the initial implementation.

## Proposal

Introduce `nfd-network-topology-updater` as a cluster-level controller. It runs
as a `Deployment`, uses leader election when configured with multiple replicas,
periodically discovers the cluster network topology, and reconciles the NFD
objects that represent that topology.

At a high level, each reconciliation performs these steps:

1. Discover or refresh a canonical cluster network topology graph.
2. Project the graph into per-node topology values.
3. Create or update one `NodeFeature` object per node with topology attributes.
4. Create or update `NodeFeatureGroup` objects for each distinct fabric tier,
   accelerator domain, and accelerator sub-domain value.
5. Delete stale updater-managed `NodeFeature` and `NodeFeatureGroup` objects
   when cleanup is enabled.

Topograph providers populate an `InstanceTopology` with:

- `FabricTiers []FabricTier` — the network switch path from the compute node
  outward. Index 0 is the tier closest to the compute node, with indexes
  increasing toward the fabric core.
- `XclrDomainID string` — an optional accelerator interconnect domain.
- `XclrSubDomainID string` — an optional accelerator sub-domain nested within
  `XclrDomainID`.

Topograph converts this provider output into a canonical `topology.Graph` whose
`Tiers` field contains the switch hierarchy and whose `Domains` field contains
accelerator or block domains. The number of fabric tiers is not fixed and can
vary between clusters or providers. Accelerator topology currently has at most
two dimensions: the domain and its optional sub-domain.

### Published NodeFeature Objects

The updater publishes one managed `NodeFeature` object per node with topology
data. Following the current Topograph NFD engine contract, the topology feature
set is named `topograph.network`; fabric elements are named `fabric-tier-N`, and
the optional accelerator elements are named `xclr-domain` and
`xclr-sub-domain`. The object name must be stable and Kubernetes-safe, combining
a normalized node name with a deterministic short hash.

Example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeature
metadata:
  name: network-topology-worker-a-...
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
      topograph.network:
        elements:
          fabric-tier-0: leaf-12
          fabric-tier-1: spine-2
          fabric-tier-2: core-1
          xclr-domain: nvl3
          xclr-sub-domain: nvl3.rack01
```

The `nfd.node.kubernetes.io/node-name` label identifies the target Kubernetes
node. The `system.name.elements.nodename` attribute also lets NFD populate group
membership when an NFD worker does not run on the node, such as for simulated
nodes. `nfd-master` can use these attributes as input when evaluating
`NodeFeatureRule` and `NodeFeatureGroup` objects.

### Published NodeFeatureGroup Objects

For each discovered topology dimension, the updater creates one
`NodeFeatureGroup` per distinct value seen in the graph. Fabric tier groups
encompass the entire discovered network fabric at that level: a `fabric-tier-0`
group contains the nodes attached to one switch at the closest fabric level,
a `fabric-tier-1` group contains the nodes reachable through its lower-tier
switches, and so on up to the outermost tier. Accelerator groups use the
`xclr-domain` and optional `xclr-sub-domain` dimensions.

NFD owns `.status.nodes`; the updater owns only object metadata and `.spec`.

Example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: network-topology-fabric-tier-0-leaf-12-...
  namespace: node-feature-discovery
  labels:
    app.kubernetes.io/managed-by: nfd-network-topology-updater
    network-topology.nfd.k8s-sigs.io/group-type: fabric-tier-0
  annotations:
    network-topology.nfd.k8s-sigs.io/label-key: fabric.topograph.run/tier-0
    network-topology.nfd.k8s-sigs.io/label-value: leaf-12
spec:
  featureGroupRules:
    - name: fabric-tier-0 equals leaf-12
      matchFeatures:
        - feature: topograph.network
          matchExpressions:
            fabric-tier-0:
              op: In
              value:
                - leaf-12
```

Group names should be stable across reconciliations. The recommended format is:

```text
network-topology-<dimension>-<normalized-value>-<hash>
```

As in Topograph, the hash should be computed from the unmodified topology value,
and the normalized value should be truncated as needed to keep the complete name
within the 63-character DNS label limit. The raw topology value should also be
stored in an annotation because normalization and truncation are lossy.

If nested `NodeFeatureGroup` support from
[kubernetes-sigs/node-feature-discovery#2551](https://github.com/kubernetes-sigs/node-feature-discovery/pull/2551)
is adopted, the updater can represent this hierarchy directly. Each
`fabric-tier-N` group would list its child `fabric-tier-(N-1)` groups rather
than restating all
lower-tier node memberships, and the outermost tier group could encompass
every node in the cluster via group nesting.

### Optional Node Labels

The Topograph Kubernetes engine publishes labels using dynamic, level-indexed
keys such as:

- `fabric.topograph.run/tier-0` (switch closest to compute)
- `fabric.topograph.run/tier-1`
- `fabric.topograph.run/tier-2`
- `accelerator.topograph.run/domain` (accelerator domain)
- `accelerator.topograph.run/sub-domain` (optional nested sub-domain)

Native Kubernetes scheduling can use this label shape naturally with pod
affinity and `topologyKey`, because the scheduler compares label values on
candidate nodes.

This KEP proposes `NodeFeature` attributes and `NodeFeatureGroup` objects as
the required MVP. Direct node label publication should be optional and disabled
by default until the project agrees on label names and trust semantics. If
enabled, the updater can request labels through `NodeFeature.spec.labels`, but
documentation must explain interactions with nfd-master restrictions such as
`restrictions.denyNodeFeatureLabels`.

## Design Details

### Topograph Integration

The implementation should keep a clear boundary between topology discovery and
NFD publication:

- Providers discover topology and return a canonical graph.
- A projection layer converts the graph into node-to-topology values.
- An NFD publisher reconciles `NodeFeature` and `NodeFeatureGroup` objects.

Topograph's current NFD engine reuses the Kubernetes engine's graph-to-label
projection, then translates the default label keys into `topograph.network`
attributes: `fabric.topograph.run/tier-N` becomes `fabric-tier-N`, and the two
accelerator keys become `xclr-domain` and `xclr-sub-domain`. The first
implementation can reuse Topograph's graph model, provider model, and projection
where licensing and dependency management allow. If the Topograph packages
cannot be imported directly, the NFD implementation should preserve the same
conceptual contract: provider output becomes a canonical graph, projection is
shared across publication backends, and the NFD publisher is independent of the
provider.

### Component Layout

The implementation should add:

- `cmd/nfd-network-topology-updater`
- `pkg/nfd-network-topology-updater`
- internal helpers for topology graph projection and object naming
- deployment manifests for RBAC, config, service account, and deployment
- Helm values and schema entries under `networkTopologyUpdater`
- a Kustomize overlay for deploying the updater

The component should use the generated NFD client for `NodeFeature` and
`NodeFeatureGroup` objects and the Kubernetes client for node discovery.

### Configuration

Initial configuration should cover:

```yaml
core:
  sleepInterval: 60s
  noPublish: false
  leaderElection: true
  nodeSelector: {}

publish:
  namespace: node-feature-discovery
  nodeFeatures: true
  nodeFeatureGroups: true
  nodeLabels: false
  cleanup: true
  topologyFeatureName: topograph.network

provider:
  name: infiniband-k8s
  configFile: /etc/kubernetes/node-feature-discovery/network-topology.conf
```

`nodeSelector` limits the Kubernetes nodes for which topology is published.
The provider name selects a Topograph-compatible discovery backend; current
Topograph backends include cloud API, NetQ, and InfiniBand providers rather than
a provider literally named `topograph`. Provider-specific options should live
under `provider` so the core publisher does not need to understand each
discovery backend.

### Reconciliation

The reconciler should:

- ignore topology entries that do not map to a current Kubernetes node, unless
  configured to preserve them for debugging;
- remove managed objects for nodes or topology values that disappear when
  cleanup is enabled;
- store a hash of the desired object payload in an annotation and skip no-op
  updates;
- update specs and metadata only, leaving `NodeFeatureGroup.status` to
  nfd-master;
- retry transient Kubernetes API failures using normal controller backoff;
- emit events or metrics for invalid graph data, publish failures, and cleanup
  failures.

The updater should not take ownership of `NodeFeature` objects created by
`nfd-worker` or by other third-party components. All list and cleanup
operations must use a strict managed-by label selector.

### RBAC

The updater service account needs permission to:

- get, list, and watch `nodes`;
- get, list, watch, create, update, patch, and delete `nodefeatures` in the
  configured namespace;
- get, list, watch, create, update, patch, and delete `nodefeaturegroups` in the
  configured namespace when group publication is enabled;
- read provider configuration and credentials from configured `ConfigMap` and
  `Secret` objects;
- create events for publish and discovery errors.

`NodeFeatureGroup` publication requires the `NodeFeatureGroupAPI` feature gate
and CRD to be enabled in the NFD installation.

### Scheduling Caveat

`NodeFeatureGroup` objects are useful for consumers that watch NFD resources,
but they do not by themselves provide the same scheduling semantics as
Kubernetes topology labels.

For example, a scheduler using native pod affinity can set:

```yaml
topologyKey: fabric.topograph.run/tier-0
```

and Kubernetes compares the tier-0 label value on candidate nodes. With
`NodeFeatureGroup`, a consumer sees one group per tier-0 value and must choose
the right group before it can make a placement decision.

Therefore, the MVP should be described as topology publication for NFD
consumers. Scheduling integration can be added later through optional node
labels, a scheduler plugin, or a future API that lets scheduling policy refer
to `NodeFeatureGroup` membership.

### Scalability

The updater writes one `NodeFeature` object per node with topology data and up
to one `NodeFeatureGroup` per distinct value in each published fabric tier,
accelerator domain, or accelerator sub-domain. `nfd-master` then writes matching
node names into each group's status.

For `N` nodes, `T` fabric tiers, and `A` published accelerator dimensions, where
`A` is 0, 1, or 2, total `NodeFeatureGroup.status.nodes` membership is
approximately `(T + A) * N`, although membership may be concentrated in large
groups at outer tier levels.
The implementation must avoid write amplification by skipping no-op updates and
by not rewriting every object on every refresh.

This proposal would benefit from adoption of
[kubernetes-sigs/node-feature-discovery#2551](https://github.com/kubernetes-sigs/node-feature-discovery/pull/2551),
the nested `NodeFeatureGroup` KEP. Network fabric topology is naturally
hierarchical: each outer-tier switch group contains the nodes represented by
its lower-tier child switch groups, and the outermost tier group can contain
every node in the cluster. Nested groups would let those higher-tier groups reference
their child groups instead of restating every matching node in each higher-tier
`NodeFeatureGroup.status.nodes` list.

### Security

Network topology can influence workload placement. Incorrect topology data may
cause poor locality or unintended co-location. Deployments should restrict who
can configure providers and who can write updater-managed `NodeFeature` and
`NodeFeatureGroup` objects.

Provider credentials should be mounted from Kubernetes `Secret` objects and
scoped to read-only topology discovery wherever possible. The updater should
not log credentials or raw provider responses at normal verbosity.

### Upgrade And Downgrade

The updater is disabled by default. Enabling it adds managed `NodeFeature` and
`NodeFeatureGroup` objects but does not require an NFD API change.

Disabling or uninstalling the updater should leave existing NFD behavior
unchanged. Installation documentation should provide a cleanup command for
removing managed objects by label:

```bash
kubectl -n node-feature-discovery delete nodefeatures,nodefeaturegroups \
  -l app.kubernetes.io/managed-by=nfd-network-topology-updater
```

If the updater is downgraded to a version that publishes fewer tier or domain
levels, cleanup behavior should be explicit: either leave unknown managed
objects untouched or delete only objects marked with a compatible publisher
version.

## Alternatives

### Keep Topograph External

Topograph `main` already ships an NFD engine that creates `NodeFeature` and
`NodeFeatureGroup` resources. It is selected with `engine.name: nfd`, supports
the `nodeSelector` and `cleanup` parameters, and requires the deployment-scoped
`NFD_NAMESPACE` environment variable (the Helm chart exposes this as
`nfdNamespace`). Keeping Topograph as a separate deployment keeps NFD smaller,
but users must install and operate another controller to expose network
topology through NFD resources.

### Publish Only Node Labels

The updater could publish only Kubernetes node labels. This is useful for
native scheduling, but it bypasses NFD's feature and group APIs and does not
serve consumers that watch NFD custom resources.

### Extend Existing nfd-topology-updater

The existing `nfd-topology-updater` is node-local and publishes
`NodeResourceTopology` objects. Extending it to discover cluster network fabric
would mix two different reconciliation models and two different topology APIs.
A separate component keeps responsibilities clearer.

### Add A New NetworkTopology CRD

A purpose-built CRD could represent the full network graph more naturally than
`NodeFeature` and `NodeFeatureGroup`. This may be useful in the future, but it
would require new API design and new consumers. The proposed MVP uses existing
NFD APIs.

## Risks And Mitigations

- **NodeFeatureGroup is alpha.** Keep the updater optional and document the
  required feature gate. Allow feature-only publication for clusters that do not
  enable groups.
- **Provider dependency risk.** Keep the Topograph boundary behind an internal
  provider interface so NFD publication logic is not tied to one provider.
- **Large API objects.** Use no-op update detection, cleanup, and future nested
  group support to reduce status size and write amplification.
- **Incorrect topology.** Expose validation metrics, events, and a dry-run mode
  that prints planned objects without publishing.
- **Label trust.** Keep direct node label publication disabled by default until
  label names and nfd-master trust settings are documented.

## Test Plan

- Unit-test graph-to-node topology projection for variable-depth fabric tiers
  and optional accelerator domain and sub-domain values.
- Unit-test stable name generation for long, invalid, and colliding topology
  values.
- Unit-test `NodeFeature` and `NodeFeatureGroup` object generation.
- Unit-test cleanup so only updater-managed objects are removed.
- Unit-test no-op reconciliation so unchanged objects are not updated.
- Add fake-client controller tests for create, update, delete, and retry paths.
- Add a dry-run test using a static Topograph-style graph fixture.
- Add an e2e test that deploys the updater with a static provider and verifies
  that `nfd-master` populates `NodeFeatureGroup.status.nodes`.
- Add Helm and Kustomize rendering tests for RBAC, config, and deployment
  manifests.
