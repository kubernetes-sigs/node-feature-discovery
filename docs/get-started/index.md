---
title: "Get started"
layout: default
sort: 1
---

# Node Feature Discovery

Welcome to Node Feature Discovery -- a Kubernetes add-on for detecting hardware
features and system configuration!

Continue to:

- **[Introduction](introduction.md)** for more details on the
  project.

- **[Quick start](quick-start.md)** for quick step-by-step
  instructions on how to get NFD running on your cluster.

## Quick-start -- the short-short version

```bash
$ kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/nfd-master.yaml.template
  namespace/node-feature-discovery created
...

$ kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/nfd-worker-daemonset.yaml.template
  daemonset.apps/nfd-worker created

$ kubectl -n node-feature-discovery get all
  NAME                              READY   STATUS    RESTARTS   AGE
  pod/nfd-master-555458dbbc-sxg6w   1/1     Running   0          56s
  pod/nfd-worker-mjg9f              1/1     Running   0          17s
...

$ kubectl get no -o json | jq .items[].metadata.labels
  {
    "beta.kubernetes.io/arch": "amd64",
    "beta.kubernetes.io/os": "linux",
    "feature.node.kubernetes.io/cpu-cpuid.ADX": "true",
    "feature.node.kubernetes.io/cpu-cpuid.AESNI": "true",
...

```
