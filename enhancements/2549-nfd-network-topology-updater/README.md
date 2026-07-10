# KEP-2549: NFD Network Topology Updater

## Summary

Add a new optional NFD component named `nfd-network-topology-updater`.

The updater discovers cluster network topology, projects it into NFD feature
data, and maintains NFD custom resources that other components can consume. The
initial implementation is based on the Topograph project and, in particular,
the Topograph NFD engine design that converts a canonical topology graph into
per-node topology attributes and `NodeFeatureGroup` objects:

<https://github.com/NVIDIA/topograph/blob/ds-nfd/docs/design/nfd-engine-sdd.md>

This component is separate from the existing `nfd-topology-updater`. The
existing updater publishes `NodeResourceTopology` objects that describe
node-local resource and NUMA topology. `nfd-network-topology-updater` publishes
cluster network fabric topology such as accelerator domains, leaf switches,
spine switches, and core switches.

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
- Create one `NodeFeatureGroup` for each discovered fabric switch at supported
  tiers such as leaf, spine, and core, and for other topology dimensions such
  as accelerator domains.
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
4. Create or update `NodeFeatureGroup` objects for discovered fabric switches
   and other supported topology values.
5. Delete stale updater-managed `NodeFeature` and `NodeFeatureGroup` objects
   when cleanup is enabled.

The Topograph design already defines the initial topology dimensions:

- `accelerator`
- `leaf`
- `spine`
- `core`

The updater should keep this model extensible so future dimensions can be added
without redesigning the publisher.

### Published NodeFeature Objects

The updater publishes one managed `NodeFeature` object per node. The object name
must be stable and Kubernetes-safe. For short node names the implementation may
include the node name; for long or invalid names it should use a deterministic
hash.

Example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeature
metadata:
  name: network-topology-worker-a
  namespace: node-feature-discovery
  labels:
    app.kubernetes.io/managed-by: nfd-network-topology-updater
    nfd.node.kubernetes.io/node-name: worker-a
  annotations:
    network-topology.nfd.k8s-sigs.io/source: topograph
spec:
  features:
    attributes:
      network.topology:
        elements:
          accelerator: nvl3
          leaf: leaf-12
          spine: spine-2
          core: core-1
```

The `nfd.node.kubernetes.io/node-name` label identifies the target Kubernetes
node. `nfd-master` can then use these attributes as input when evaluating
`NodeFeatureRule` and `NodeFeatureGroup` objects.

### Published NodeFeatureGroup Objects

For each supported topology dimension, the updater creates one
`NodeFeatureGroup` for each distinct value seen in the graph. For network
fabric tiers, these generated switch groups encompass the entire discovered
network fabric: one group for each leaf switch, one group for each spine
switch, and one group for each core switch. A leaf group contains the nodes
attached under that leaf switch, a spine group contains the nodes reachable
through its lower-tier leaf switches, and a top-level core switch group can
encompass every node in the discovered cluster fabric.

NFD owns `.status.nodes`; the updater owns only object metadata and `.spec`.

Example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: network-topology-leaf-x3f91c4a0
  namespace: node-feature-discovery
  labels:
    app.kubernetes.io/managed-by: nfd-network-topology-updater
    network-topology.nfd.k8s-sigs.io/dimension: leaf
  annotations:
    network-topology.nfd.k8s-sigs.io/source: topograph
    network-topology.nfd.k8s-sigs.io/value: leaf-12
spec:
  featureGroupRules:
    - name: leaf-12-branch
      matchFeatures:
        - feature: network.topology
          matchExpressions:
            leaf:
              op: In
              value:
                - leaf-12
```

Group names should be stable across reconciliations. The recommended format is:

```text
network-topology-<dimension>-<hash>
```

The raw topology value should be stored in annotations rather than directly in
the object name, because fabric object names may contain characters that are not
valid in Kubernetes names or may be too long.

If nested `NodeFeatureGroup` support from
[kubernetes-sigs/node-feature-discovery#2551](https://github.com/kubernetes-sigs/node-feature-discovery/pull/2551)
is adopted, the updater can represent this hierarchy directly. Leaf switch
groups would remain the lowest-tier groups, while spine switch groups could
list their child leaf groups and core switch groups could list their child
spine groups. This avoids restating all lower-tier node memberships in every
higher-tier switch group.

### Optional Node Labels

The Topograph Kubernetes engine publishes labels such as:

- `network.topology.nvidia.com/accelerator`
- `network.topology.nvidia.com/leaf`
- `network.topology.nvidia.com/spine`
- `network.topology.nvidia.com/core`

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

The first implementation can reuse Topograph's graph model, provider model, and
graph-to-topology projection where licensing and dependency management allow.
If the Topograph packages cannot be imported directly, the NFD implementation
should preserve the same conceptual contract: provider output becomes a
canonical graph, and the NFD publisher is independent of the provider.

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
  topologyFeatureName: network.topology

provider:
  name: topograph
  configFile: /etc/kubernetes/node-feature-discovery/network-topology.conf
```

`nodeSelector` limits the Kubernetes nodes for which topology is published.
Provider-specific options should live under `provider` so the core publisher
does not need to understand each discovery backend.

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
topologyKey: network.topology.nvidia.com/leaf
```

and Kubernetes compares the leaf label value on candidate nodes. With
`NodeFeatureGroup`, a consumer sees one group per leaf value and must choose the
right group before it can make a placement decision.

Therefore, the MVP should be described as topology publication for NFD
consumers. Scheduling integration can be added later through optional node
labels, a scheduler plugin, or a future API that lets scheduling policy refer
to `NodeFeatureGroup` membership.

### Scalability

The updater writes one `NodeFeature` object per node and up to one
`NodeFeatureGroup` per distinct value in each published topology dimension.
`nfd-master` then writes matching node names into each group's status.

For `N` nodes and four dimensions, total `NodeFeatureGroup.status.nodes`
membership is approximately `4 * N`, although membership may be concentrated in
large groups such as a core switch containing many nodes. The implementation
must avoid write amplification by skipping no-op updates and by not rewriting
every object on every refresh.

This proposal would benefit from adoption of
[kubernetes-sigs/node-feature-discovery#2551](https://github.com/kubernetes-sigs/node-feature-discovery/pull/2551),
the nested `NodeFeatureGroup` KEP. Network fabric topology is naturally
hierarchical: each non-leaf switch group contains the nodes represented by its
lower-tier child switch groups, and the top switch group can contain every node
in the cluster. Nested groups would let those higher-tier groups reference
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

If the updater is downgraded to a version that does not support a newer topology
dimension, cleanup behavior should be explicit: either leave unknown managed
objects untouched or delete only objects marked with a compatible publisher
version.

## Alternatives

### Keep Topograph External

Topograph can remain a separate deployment with its own NFD engine. This keeps
NFD smaller, but users must install and operate another controller to expose
network topology through NFD resources.

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

- Unit-test graph-to-node topology projection for accelerator, leaf, spine, and
  core dimensions.
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
