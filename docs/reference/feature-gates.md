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

| Name                  | Default | Stage  | Since   | Until  |
| --------------------- | ------- | ------ | ------- | ------ |
| `NodeFeatureAPI`      | true    | Beta   | V0.14   | v0.16  |
| `NodeFeatureAPI`      | true    | GA     | V0.17   |        |
| `DisableAutoPrefix`   | false   | Alpha  | V0.16   |        |
| `NodeFeatureGroupAPI` | false   | Alpha  | V0.16   |        |

## NodeFeatureAPI

The `NodeFeatureAPI` feature gate enables the Node Feature API.
When enabled, NFD will register the Node Feature API with the Kubernetes API
server. The Node Feature API is used to expose node-specific hardware and
software features to the Kubernetes scheduler. The Node Feature API is a beta
feature and is enabled by default.

## NodeFeatureGroupAPI

The `NodeFeatureGroupAPI` feature gate enables the Node Feature Group API.
When enabled, NFD will register the Node Feature Group API with the Kubernetes API
server. The Node Feature Group API is used to create node groups based on
hardware and software features. The Node Feature Group API is an alpha feature
and is disabled by default.

## DisableAutoPrefix

The `DisableAutoPrefix` feature gate controls the automatic prefixing of names.
When enabled nfd-master does not automatically add the default
`feature.node.kubernetes.io/` prefix to unprefixed labels, annotations and
extended resources. Automatic prefixing is the default behavior in NFD v0.16
and earlier.

Note that enabling the feature gate effectively causes unprefixed names to be
filtered out as NFD does not allow unprefixed names of labels, annotations or
extended resources. For example, with the `DisableAutoPrefix` feature gate set
to `false`, a NodeFeatureRule with

```yaml
  labels:
    foo: bar
```

will be automatically prefixed, resulting in the node label
`feature.node.kubernetes.io/foo=bar`. However, when `DisableAutoPrefix` is set
to `true`, no prefix is added, and the label remains as `foo=bar`. Note that
taint keys are not affected by this feature gate.
