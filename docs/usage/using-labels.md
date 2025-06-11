---
title: "Using node labels"
parent: "Usage"
layout: default
nav_order: 2
---

# Using node labels
{: .no_toc}

---

Nodes with specific features can be targeted using the `nodeSelector` field. The
following example shows how to target nodes with Intel TurboBoost enabled.

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    env: test
  name: golang-test
spec:
  containers:
    - image: golang
      name: go1
  nodeSelector:
    feature.node.kubernetes.io/cpu-pstate.turbo: 'true'
```

For more details on targeting nodes, see
[node selection](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).
