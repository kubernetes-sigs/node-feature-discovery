# KEP-2548: Nested NodeFeatureGroups

## Summary

`NodeFeatureGroup` stores all matching nodes in `.status.nodes`. In large
clusters, a broadly matching group can grow until the object becomes expensive
to watch and, in the worst case, too large for the backing Kubernetes storage.

This enhancement proposes support for nested `NodeFeatureGroup` objects. A
group may include one or more existing groups by name and expose those
references in status instead of always expanding every member node into the
parent status. The parent group's effective membership is the union of its
directly matched nodes and the effective membership of the referenced groups.

The proposed API uses `containsGroups` in `featureGroupRules` and `groups` in
status.

## Motivation

`NodeFeatureGroup` is designed for grouping nodes by their discovered features.
Today, each group is materialized as a flat list of node names. That model is
simple, but it scales poorly for groups that match a large share of a cluster.

Kubernetes object names that use DNS subdomain naming can be up to 253
characters. In practice, cloud provider node names are often tens of characters
long and can reach 63-128 characters. etcd is designed for small metadata
records and defaults to a maximum request size of 1.5 MiB. At 128 characters per
node name, 1.5 MiB contains about 12k raw node names before JSON serialization,
field names, object metadata, managed fields, and status overhead are counted.
Clusters already exist beyond that size, so a `NodeFeatureGroup` that matches
all or most nodes can become too large to update reliably.

Large `NodeFeatureGroup` objects also increase apiserver, etcd, informer, and
scheduler I/O. Even when an object fits under storage limits, repeatedly reading
and watching a large status payload is inefficient.

### Goals

- Allow a `NodeFeatureGroup` to include existing `NodeFeatureGroup` objects by
  name.
- Reduce duplication in `NodeFeatureGroup.status.nodes` when groups are built
  from other reusable groups.
- Preserve a clear effective-membership model: a parent group contains the
  union of directly matched nodes and referenced groups.
- Detect and reject invalid references, including missing groups and cycles.
- Keep the feature compatible with the existing `NodeFeatureGroupAPI` feature
  gate and current `featureGroupRules` behavior.

### Non-Goals

- Changing the meaning of existing `matchFeatures`, `matchAny`, `vars`, or
  `varsTemplate` fields.
- Introducing set operations other than union, such as intersection or
  subtraction.
- Replacing `.status.nodes` for groups that do not reference other groups.
- Sharding one logical `NodeFeatureGroup` across multiple Kubernetes objects.
- Adding scheduler-specific policy in this enhancement.

## Proposal

Extend `GroupRule` with an optional list of referenced group names:

```go
type GroupRule struct {
    // Existing fields omitted.

    // ContainsGroups specifies NodeFeatureGroups whose effective node
    // membership is included in this rule.
    // +optional
    // +listType=set
    // +kubebuilder:validation:items:MaxLength=253
    ContainsGroups []string `json:"containsGroups,omitempty"`
}
```

Extend `NodeFeatureGroupStatus` with referenced groups and a total node count:

```go
type NodeFeatureGroupStatus struct {
    // Nodes is the list of directly materialized nodes in this group status.
    Nodes []FeatureGroupNode `json:"nodes"`

    // Groups is the list of NodeFeatureGroups whose effective membership is
    // included in this group.
    // +optional
    // +listType=map
    // +listMapKey=name
    Groups []FeatureGroupReference `json:"groups,omitempty"`

    // NodeCount is the effective number of unique nodes in this group,
    // including nodes represented by referenced groups.
    // +optional
    // +kubebuilder:validation:Minimum=0
    NodeCount int64 `json:"nodeCount,omitempty"`
}

type FeatureGroupReference struct {
    // Name of the referenced NodeFeatureGroup.
    // +kubebuilder:validation:MaxLength=253
    Name string `json:"name"`
}
```

Add the same `+kubebuilder:validation:MaxLength=253` marker to the existing
`FeatureGroupNode.Name` field. Node and `NodeFeatureGroup` names use DNS
subdomain naming, which allows at most 253 characters. `containsGroups` is a
set, so duplicate names within a rule are rejected. `status.groups` is a map
list keyed by name, matching the existing `status.nodes` list semantics, so
duplicate group entries are rejected. `status.nodeCount` must be nonnegative.
References to the same group from different rules still contribute its members
only once to the effective membership.

For compact YAML, the API may also use `groups: []string` in status. A structured
reference type is preferred because it leaves room for future fields such as
`nodeCount`, `observedGeneration`, or readiness state without another status API
change.

### Example

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: vendor-a
spec:
  featureGroupRules:
    - name: vendor-a-devices
      matchFeatures:
        - feature: pci.device
          matchExpressions:
            vendor:
              op: In
              value:
                - "a"
status:
  nodeCount: 4
  nodes:
    - name: worker-a-01
    - name: worker-a-02
    - name: worker-a-03
    - name: worker-a-04
```

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: vendor-ab
spec:
  featureGroupRules:
    - name: vendor-b-devices
      matchFeatures:
        - feature: pci.device
          matchExpressions:
            vendor:
              op: In
              value:
                - "b"
    - name: include-vendor-a
      containsGroups:
        - vendor-a
status:
  nodeCount: 7
  nodes:
    - name: worker-b-01
    - name: worker-b-02
    - name: worker-b-03
  groups:
    - name: vendor-a
```

The effective membership of `vendor-ab` is:

- `worker-b-01`, `worker-b-02`, and `worker-b-03` from its direct rule.
- all effective members of `vendor-a`, represented by the `groups` status entry.

### API Semantics

Each entry in `.spec.featureGroupRules` continues to contribute to the group by
logical union. A rule matches a node when its existing feature-match fields
match that node. A rule with `containsGroups` and no feature-match fields
matches no node directly. Today's evaluator matches every node with a
`NodeFeature` when `matchFeatures` and `matchAny` are both empty, so the
controller must check `containsGroups` first. Rules without `containsGroups`
keep today's behavior. A rule with `containsGroups` contributes the effective
membership of each referenced group.

If a rule contains both feature-match fields and `containsGroups`, the
contributions are unioned. This keeps the behavior consistent with the current
rule list, where matching any rule adds the node to the group.

References are namespace-local because `NodeFeatureGroup` is namespaced. A
reference to `vendor-a` from group `vendor-ab` resolves to a
`NodeFeatureGroup` named `vendor-a` in the same namespace.

The controller must de-duplicate effective members when computing `nodeCount`.
If a node is included directly and through one or more referenced groups, it is
counted once.

### Status Semantics

`status.nodes` contains nodes directly matched by this group after excluding
nodes that can be represented by a referenced group in `status.groups`. The
controller should prefer compact status when a node is already included by a
referenced group.

`status.groups` contains direct group references included by this group. It is
not required to flatten transitive group references into the parent status.

`status.nodeCount` contains the effective unique node count. Consumers that only
need cardinality can read `nodeCount` without expanding references.

Consumers that need the full node set must recursively resolve `status.groups`
and union those groups' effective members with `status.nodes`. They must also
watch every group that they resolve, because a membership change in a
referenced group does not always change the parent's status.

### Conditions

Add `status.conditions` if the API already adopts Kubernetes-style status
conditions before or during implementation. Otherwise, include invalid
references in controller events and logs and leave the last valid status in
place.

Recommended conditions:

- `Ready`: the group status reflects the latest observed generation and all
  references were resolved.
- `ReferencesResolved`: all referenced groups exist and can be traversed.
- `CycleDetected`: the group participates in a reference cycle.

## Design Details

### Controller Evaluation

The nfd-master controller currently evaluates each `NodeFeatureGroup` by
iterating over nodes, executing all `featureGroupRules`, and writing the
matching node list to status.

With nested groups, evaluation should be split into two phases:

1. Evaluate direct feature rules and produce the direct node set for every
   `NodeFeatureGroup`.
2. Resolve group references as a directed graph and compute effective
   membership, `status.groups`, and `status.nodeCount`.

The graph edge direction is `parent -> referenced group`. If a group references
a missing group or a cycle is detected, the controller should mark that group
invalid and avoid publishing a misleading status update for it.

Creating, deleting, or changing a `NodeFeatureGroup` can affect groups that
reference it, directly or transitively. Changes in direct node membership can
also affect those parents. The controller must reconcile the affected groups
when any of these changes occurs.

The controller can maintain a namespace-local reverse-reference index from each
referenced group name to its direct parents. The index is derived from parent
specs, including references to groups that do not yet exist. On a group create,
delete, or relevant update, the controller can traverse the index to enqueue all
transitive parents, tracking visited groups so cycles do not cause an infinite
traversal. Reconciliation resolves the current graph and recomputes membership
and counts; if a reference is missing or cyclic, the affected groups retain
their last valid status as described below. When a parent spec changes or the
parent is deleted, its entries in the index must be updated or removed.

A simple initial implementation may instead enqueue all groups on these events;
the reverse-reference index is an optimization and must preserve the same
behavior.

The existing updater pool can reconcile groups concurrently. Enqueueing a child
and its parents therefore does not guarantee that the child is evaluated first.
The implementation must define how a parent obtains a current, complete view
of its dependencies and how it retries when those dependencies change during
reconciliation. Merely reading a child's last published status can produce an
incorrect parent membership or count. The consistency guarantee and the choice
between computing membership from the graph or coordinating child status
updates remain to be resolved before implementation.

### Cycle Handling

Cycles are invalid. For example:

```yaml
a -> b -> c -> a
```

The controller should detect cycles during graph traversal and report the full
cycle path in logs and events. Groups in the cycle should not be updated with
partially resolved status. Their parents should be treated as unresolved until
the cycle is fixed.

Self-reference is a cycle and is invalid.

### Missing References

If a referenced group does not exist, the referencing group is unresolved. The
controller should report the missing reference and leave the last valid status
unchanged. This prevents transient deletions or apply-order issues from
publishing a partial group that could be interpreted as complete.

### RBAC and Security

This enhancement does not require new RBAC permissions beyond the existing
ability for nfd-master to watch and update `NodeFeatureGroup` resources in its
namespace.

Because references are namespace-local, a tenant cannot include groups from
another namespace through this field.

### Feature Gate

The feature should remain under the existing `NodeFeatureGroupAPI` feature gate
while the entire API is alpha. A separate feature gate is not required unless
maintainers want staged rollout inside the alpha API.

### Backward Compatibility

Existing `NodeFeatureGroup` objects without `containsGroups` continue to behave
as they do today. Their status may gain `nodeCount`, but `status.nodes` remains
the authoritative flat list for groups with no nested references.

Existing clients that read only `status.nodes` will continue to work for
non-nested groups. Clients that need correct membership for nested groups must
be updated to resolve `status.groups`.

### Upgrade and Downgrade

On upgrade, nfd-master adds `status.nodeCount` to each existing group that has
members, in one status update. `status.nodes` and group membership do not
change until users add `containsGroups`.

On downgrade to a version that does not understand `containsGroups` or
`status.groups`, nested groups will not be evaluated correctly. Documentation
should instruct users to remove nested references and wait for status to be
rewritten before downgrading.

## Risks and Mitigations

- **Consumers may ignore `status.groups`.** Document that nested groups require
  recursive resolution for full membership and provide helper code where NFD
  consumers exist in-tree.
- **Cycles can make membership ambiguous.** Reject cycles and preserve the last
  valid status instead of publishing partial results.
- **Reference churn can trigger broad recomputation.** Start with correct
  recomputation, then add a reverse-reference index if needed.
- **Deep nesting can increase read amplification.** Document an implementation
  limit for maximum reference depth and expose events when the limit is reached.
- **`nodeCount` can be stale if referenced groups are stale.** Track observed
  generations in memory during reconciliation and, if conditions are added,
  expose readiness in status.

## Alternatives

### Status Sharding

The controller could split one logical group into several child status objects.
This would avoid changing membership semantics for consumers but would add a new
resource model and more objects for users and controllers to manage.

### Always Expand Parent Status

The controller could let users reference groups in spec but still expand the
full node list into the parent status. This would improve authoring ergonomics
but would not solve the object-size problem.

### Label-Based Selection

Users can sometimes represent groups by labels and selectors. That approach does
not cover all `NodeFeatureGroup` use cases because groups are derived from NFD
feature rules and may be consumed as explicit node pools.

### Set Operations

Intersection and subtraction would make group composition more expressive, but
they are not needed to address the immediate scaling problem and would require a
larger API design.

## Test Plan

- Unit tests for API defaulting, deepcopy, generated clients, and CRD schema
  validation for `containsGroups`, `status.groups`, and `status.nodeCount`.
- CRD validation tests for names at the 253-character limit and over it,
  duplicate names in `containsGroups`, `status.groups`, and `status.nodes`,
  and a negative `status.nodeCount`.
- Unit tests for graph resolution:
  - single-level references
  - transitive references
  - duplicate nodes across direct and referenced groups
  - missing references
  - self-reference
  - multi-group cycles
  - maximum-depth enforcement
- Controller tests that verify child creation, deletion, spec updates, and direct
  membership changes reconcile transitive parents. Cover a parent created before
  its child, preservation of the last valid status after child deletion, and
  recovery when the child is recreated.
- Tests for reverse-reference index maintenance when parent references change or
  parents are deleted, and for traversal of a cyclic graph without looping.
- Controller tests with concurrent group workers that verify parents converge
  to the correct membership and count even when reconciled before their children.
- E2E test with two directly matched groups and one nested group, verifying
  compact status and effective `nodeCount`.
- Large synthetic test that verifies a nested parent can represent membership
  that would otherwise require a very large flat `status.nodes` list.

## Graduation Criteria

### Alpha

- API fields are implemented under the existing `NodeFeatureGroupAPI` alpha
  feature gate.
- Controller resolves namespace-local references and rejects cycles.
- Documentation explains status resolution and client expectations.

### Beta

- Status conditions or equivalent machine-readable error reporting are
  available.
- Existing in-tree consumers correctly resolve nested groups.
- Scale testing demonstrates reduced status payload size for composed groups.

## References

- Node Feature Discovery issue list:
  https://github.com/kubernetes-sigs/node-feature-discovery/issues
- Kubernetes object name constraints:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
- etcd request size limit:
  https://etcd.io/docs/v3.6/dev-guide/limit/#request-size-limit
