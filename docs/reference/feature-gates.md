---
title: "Feature Gates"
layout: default
sort: 11
---

# Feature Gates
{: .no_toc}

---

Feature gates are a set of key-value pairs that control the behavior of NFD.
They are used to enable or disable certain features of NFD.
The feature gates are set using the `-feature-gates` command line flag or
`featureGates` value in the Helm chart. The following feature gates are available:

| Name                  | Default       | Stage  | Since   | Until  |
| --------------------- | ------------- | ------ | ------- | ------ |
| `NodeFeatureAPI`      | true          | Beta   | v0.14   | v0.16  |
| `NodeFeatureAPI`      | true (locked) | GA     | v0.17   |        |
| `DisableAutoPrefix`   | false         | Alpha  | v0.16   |        |
| `NodeFeatureGroupAPI` | false         | Alpha  | v0.16   |        |

## NodeFeatureAPI

The `NodeFeatureAPI` feature gate enables the NodeFeature API (the
`NodeFeature` custom resource) as the communication channel between nfd-worker
and nfd-master. The feature gate is GA since NFD v0.17 and locked to `true`:
setting `-feature-gates=NodeFeatureAPI=false` fails with "cannot set feature
gate NodeFeatureAPI to false, feature is locked to true". The gate will be
removed in a future release. The NFD custom resource definitions are installed
by the deployment (the Helm chart or the kustomize base), not by NFD itself.

## NodeFeatureGroupAPI

The `NodeFeatureGroupAPI` feature gate enables processing of NodeFeatureGroup
objects in nfd-master. When enabled, nfd-master evaluates the rules of each
NodeFeatureGroup in its own namespace and writes the matching nodes to the
object's status. The NodeFeatureGroup CRD is installed by the deployment
regardless of this gate; with the gate disabled such objects can be created but
their status is never updated. The Node Feature Group API is an alpha feature
and is disabled by default.

## DisableAutoPrefix

The `DisableAutoPrefix` feature gate controls the automatic prefixing of names.
When enabled nfd-master does not automatically add the default
`feature.node.kubernetes.io/` prefix to unprefixed labels, annotations and
extended resources. Automatic prefixing is the default behavior in NFD v0.16
and earlier.

For example, with the `DisableAutoPrefix` feature gate set to `false`, a
NodeFeatureRule with

```yaml
  labels:
    foo: bar
```

will be automatically prefixed, resulting in the node label
`feature.node.kubernetes.io/foo=bar`. However, when `DisableAutoPrefix` is set
to `true`, no prefix is added, and the label remains as `foo=bar`. Note that
taint keys are not affected by this feature gate.

Note: nfd-master does not remove the unprefixed labels, annotations and
extended resources that it created while this gate was enabled when they are no
longer produced, for example when the NodeFeatureRule is deleted or changed, or
when the gate is set back to `false`. They stay on the node until removed
manually:

```bash
kubectl label node <node> <key>-
kubectl annotate node <node> <key>-
kubectl patch node <node> --subresource=status --type=json \
  -p '[{"op":"remove","path":"/status/capacity/<name>"},{"op":"remove","path":"/status/allocatable/<name>"}]'
```
