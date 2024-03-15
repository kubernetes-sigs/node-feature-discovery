---
title: "Feature Gates"
layout: default
sort: 10
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
| `NodeFeatureAPI`      | true    | Beta   | V0.14   |   |

## NodeFeatureAPI

The `NodeFeatureAPI` feature gate enables the Node Feature API.
When enabled, NFD will register the Node Feature API with the Kubernetes API
server. The Node Feature API is used to expose node-specific hardware and
software features to the Kubernetes scheduler. The Node Feature API is a beta
feature and is enabled by default.
